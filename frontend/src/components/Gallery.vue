<template>
  <section class="card">
    <div
      v-if="store.settings.source === 'telegram'"
      class="settings-section-divider"
    >
      <h3>Telegram Mode Active</h3>
      <p>
        The frame is currently displaying photos sent to your Telegram Bot.
        <br />
        Go to <b>Settings</b> to switch back to Google Photos mode.
      </p>
    </div>

    <div v-else>
      <div
        class="gallery-header"
        style="
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 20px;
        "
      >
        <h2>Photo Gallery</h2>
        <button
          @click="startPicker"
          :disabled="loading"
          class="btn btn-primary"
        >
          <span v-if="loading">Processing...</span>
          <span v-else>Add Photos via Google</span>
        </button>
      </div>

      <!-- Notification -->
      <div
        v-if="importMessage"
        class="status-message"
        :class="{
          'status-error':
            importMessage.includes('Error') || importMessage.includes('Failed'),
          'status-success':
            !importMessage.includes('Error') &&
            !importMessage.includes('Failed'),
        }"
      >
        {{ importMessage }}
      </div>

      <!-- Photo Grid -->
      <div v-if="photos.length > 0" class="image-grid">
        <div v-for="photo in photos" :key="photo.id" class="image-item group">
          <img
            :src="photo.thumbnail_url"
            :alt="photo.caption"
            class="image-thumbnail"
            style="
              width: 100%;
              height: auto;
              object-fit: contain;
              max-width: 100%;
            "
            loading="lazy"
          />

          <!-- Delete Button -->
          <button
            @click="deletePhoto(photo.id)"
            class="delete-btn"
            title="Delete Photo"
          >
            ×
          </button>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="loading">
        <h3>No photos</h3>
        <p>Get started by adding photos from Google Photos.</p>
        <div style="margin-top: 20px">
          <button @click="startPicker" class="btn btn-primary">
            Add Photos
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useSettingsStore } from '../stores/settings';

const store = useSettingsStore();
const photos = ref<any[]>([]);
const loading = ref(false);

// Fetch Photos
const fetchPhotos = async () => {
  try {
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE || ''}/api/photos`
    );
    if (res.ok) {
      const data = await res.json();
      photos.value = data || [];
    }
  } catch (e) {
    console.error('Failed to fetch photos', e);
  }
};

// Delete Photo
const deletePhoto = async (id: number) => {
  if (!confirm('Are you sure you want to delete this photo?')) return;

  try {
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE || ''}/api/photos/${id}`,
      {
        method: 'DELETE',
      }
    );
    if (res.ok) {
      photos.value = photos.value.filter((p) => p.id !== id);
    } else {
      alert('Failed to delete photo');
    }
  } catch (e) {
    console.error('Failed to delete photo', e);
  }
};

// Picker Logic (Simplified from Settings.vue)
const pickerTimer = ref<number | null>(null);

const startPicker = async () => {
  // Check Credentials
  if (
    !store.settings.google_client_id ||
    !store.settings.google_client_secret
  ) {
    importMessage.value =
      'Please configure Google Photos Credentials in Settings first.';
    setTimeout(() => (importMessage.value = ''), 5000);
    return;
  }

  loading.value = true;
  try {
    // 1. Create Session
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE || ''}/api/google/picker/session`
    );
    if (!res.ok) throw new Error('Failed to create session');
    const { id, pickerUri } = await res.json();

    // 2. Open Popup
    const width = 800;
    const height = 600;
    const left = (window.screen.width - width) / 2;
    const top = (window.screen.height - height) / 2;
    window.open(
      pickerUri,
      'GooglePicker',
      `width=${width},height=${height},top=${top},left=${left}`
    );

    // 3. Poll for completion
    pollPicker(id);
  } catch (e) {
    console.error(e);
    importMessage.value = 'Failed to start picker flow';
    loading.value = false;
  }
};

const pollPicker = (sessionId: string) => {
  if (pickerTimer.value) clearInterval(pickerTimer.value);

  pickerTimer.value = window.setInterval(async () => {
    try {
      const res = await fetch(
        `${import.meta.env.VITE_API_BASE || ''}/api/google/picker/poll/${sessionId}`
      );
      if (res.ok) {
        const { complete } = await res.json();
        if (complete) {
          // Stop polling
          if (pickerTimer.value) clearInterval(pickerTimer.value);

          // Trigger Sync
          await processPicker(sessionId);
        }
      }
    } catch (e) {
      console.error('Polling error', e);
    }
  }, 2000);
};

const importMessage = ref('');

const processPicker = async (sessionId: string) => {
  try {
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE || ''}/api/google/picker/process/${sessionId}`,
      {
        method: 'POST',
      }
    );

    if (res.status === 202) {
      // Poll Progress
      const progressInterval = setInterval(async () => {
        try {
          const pRes = await fetch(
            `${import.meta.env.VITE_API_BASE || ''}/api/google/picker/progress/${sessionId}`
          );
          if (pRes.ok) {
            const pData = await pRes.json();
            // Refresh gallery periodically to show progress
            fetchPhotos();

            if (pData.status === 'done') {
              clearInterval(progressInterval);
              importMessage.value = `Successfully added ${pData.processed} photos!`;
              setTimeout(() => (importMessage.value = ''), 5000);
              loading.value = false;
            } else if (pData.status === 'error') {
              clearInterval(progressInterval);
              importMessage.value = `Error: ${pData.error}`;
              loading.value = false;
            }
          }
        } catch (e) {
          console.error('Progress poll error', e);
        }
      }, 2000);
    } else if (res.ok) {
      const { count } = await res.json();
      importMessage.value = `Successfully added ${count} photos!`;
      setTimeout(() => (importMessage.value = ''), 5000);
      fetchPhotos(); // Refresh list
      loading.value = false;
    } else {
      importMessage.value = 'Failed to process photos';
      loading.value = false;
    }
  } catch (e) {
    console.error('Process error', e);
    importMessage.value = 'Error processing photos';
    loading.value = false;
  }
};

onMounted(async () => {
  await store.fetchSettings();
  fetchPhotos();
});
</script>

<style scoped>
.status-success {
  color: #059669;
  background-color: #d1fae5;
  padding: 10px;
  border-radius: 4px;
  margin-bottom: 20px;
  width: 100%;
}

.status-error {
  color: #dc2626;
  background-color: #fee2e2;
  padding: 10px;
  border-radius: 4px;
  margin-bottom: 20px;
  width: 100%;
}
</style>
