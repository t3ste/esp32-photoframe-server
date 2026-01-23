<template>
  <section class="card">
    <h2>Settings</h2>

    <div v-if="store.loading" class="loading">Loading...</div>
    <div v-else-if="store.error" class="status-error">{{ store.error }}</div>

    <form @submit.prevent="save">
      <!-- Device Configuration -->
      <div class="settings-section-divider">
        <h3 class="section-subtitle">Device Configuration</h3>

        <div class="form-group">
          <label>Display Orientation:</label>
          <div class="rotation-mode-group">
            <label class="radio-label">
              <input
                type="radio"
                value="landscape"
                v-model="form.Orientation"
              />
              <span>Landscape</span>
            </label>
            <label class="radio-label">
              <input type="radio" value="portrait" v-model="form.Orientation" />
              <span>Portrait</span>
            </label>
          </div>
        </div>

        <div class="form-group">
          <label>Dimensions:</label>
          <div style="display: flex; gap: 10px">
            <div class="time-input-field">
              <span class="time-input-label">Width</span>
              <input type="number" v-model="form.DisplayWidth" />
            </div>
            <div class="time-input-field">
              <span class="time-input-label">Height</span>
              <input type="number" v-model="form.DisplayHeight" />
            </div>
          </div>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="form.CollageMode" />
            <span>Enable Collage Mode (Combine 2 photos)</span>
          </label>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="form.show_date" />
            <span>Show Date</span>
          </label>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="form.show_weather" />
            <span>Show Weather</span>
          </label>
        </div>
      </div>

      <!-- Photo Source -->
      <div class="settings-section-divider">
        <h3 class="section-subtitle">Data Source</h3>
        <div class="form-group">
          <label>Source:</label>
          <div class="rotation-mode-group">
            <label class="radio-label">
              <input type="radio" value="google_photos" v-model="form.source" />
              <span>Google Photos</span>
            </label>
            <label class="radio-label">
              <input type="radio" value="telegram" v-model="form.source" />
              <span>Telegram Bot</span>
            </label>
          </div>
        </div>

        <div class="form-group" v-if="form.source === 'telegram'">
          <label>Telegram Bot Token</label>
          <input
            type="text"
            v-model.trim="form.telegram_bot_token"
            placeholder="Enter Bot Token"
          />
          <p class="helper-text">
            Send photos to your bot to display them. Only the last photo will be
            shown.
          </p>
        </div>
      </div>

      <!-- Google Photos Integration -->
      <div
        class="settings-section-divider"
        v-if="form.source === 'google_photos'"
      >
        <h3 class="section-subtitle">Google Photos Integration</h3>

        <div class="info-box">
          <div class="info-box-title">ℹ️ Setup Required</div>
          <p class="info-box-caption">
            To enable Google Photos, create a project in
            <a href="https://console.cloud.google.com/" target="_blank"
              >Google Cloud Console</a
            >.
            <br />
            Redirect URI:
            <code>http://[YOUR_SERVER_IP]:8080/api/auth/google/callback</code>
          </p>
        </div>

        <div class="form-group">
          <label>Client ID</label>
          <input type="text" v-model.trim="form.google_client_id" />
        </div>

        <div class="form-group">
          <label>Client Secret</label>
          <input type="password" v-model.trim="form.google_client_secret" />
        </div>

        <div class="form-group">
          <button
            v-if="form.google_client_id && form.google_client_secret"
            @click.prevent="connectGoogle"
            type="button"
            class="btn btn-primary"
            style="width: 100%; margin-bottom: 20px"
          >
            Authorize with Google
          </button>
          <span v-else class="helper-text"
            >Save Client ID/Secret to enable connection.</span
          >
        </div>
      </div>

      <!-- Weather Overlay -->
      <div class="settings-section-divider">
        <h3 class="section-subtitle">Weather Overlay</h3>
        <div class="form-group">
          <label>Location:</label>
          <div style="display: flex; gap: 10px">
            <div style="flex: 1">
              <span class="field-label">Latitude</span>
              <input type="text" v-model="form.weather_lat" />
            </div>
            <div style="flex: 1">
              <span class="field-label">Longitude</span>
              <input type="text" v-model="form.weather_lon" />
            </div>
          </div>
        </div>
      </div>

      <div class="settings-save-section">
        <button type="submit" class="btn btn-primary" :disabled="store.loading">
          {{ store.loading ? 'Saving...' : 'Save All Settings' }}
        </button>
        <div
          v-if="saveMessage"
          class="status-message"
          :class="{
            'status-error': saveMessage.includes('Error'),
            'status-success': !saveMessage.includes('Error'),
          }"
        >
          {{ saveMessage }}
        </div>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { useSettingsStore } from '../stores/settings';

const store = useSettingsStore();

const form = reactive({
  Orientation: 'landscape',
  DisplayWidth: '800',
  DisplayHeight: '480',
  CollageMode: false,
  show_date: true,
  show_weather: true,
  google_client_id: '',
  google_client_secret: '',
  source: 'google_photos',
  telegram_bot_token: '',
  weather_lat: '',
  weather_lon: '',
  selected_albums: [] as string[],
});

onMounted(async () => {
  await store.fetchSettings();
  Object.assign(form, {
    Orientation: store.settings.orientation || 'landscape',
    DisplayWidth: store.settings.display_width || '800',
    DisplayHeight: store.settings.display_height || '480',
    CollageMode: store.settings.collage_mode === 'true',
    show_date: store.settings.show_date !== 'false', // Default true
    show_weather: store.settings.show_weather !== 'false', // Default true
    google_client_id: store.settings.google_client_id || '',
    google_client_secret: store.settings.google_client_secret || '',
    source: store.settings.source || 'google_photos',
    telegram_bot_token: store.settings.telegram_bot_token || '',
    weather_lat: store.settings.weather_lat || '',
    weather_lon: store.settings.weather_lon || '',
    selected_albums: store.settings.selected_albums
      ? JSON.parse(store.settings.selected_albums)
      : [],
  });
});

const saveSettingsInternal = async () => {
  await store.saveSettings({
    orientation: form.Orientation,
    display_width: String(form.DisplayWidth),
    display_height: String(form.DisplayHeight),
    collage_mode: String(form.CollageMode),
    show_date: String(form.show_date),
    show_weather: String(form.show_weather),
    google_client_id: form.google_client_id,
    google_client_secret: form.google_client_secret,
    source: form.source,
    telegram_bot_token: form.telegram_bot_token,
    weather_lat: form.weather_lat,
    weather_lon: form.weather_lon,
    selected_albums: JSON.stringify(form.selected_albums),
  });
};

const saveMessage = ref('');

const save = async () => {
  try {
    await saveSettingsInternal();
    saveMessage.value = 'Settings saved successfully!';
    setTimeout(() => {
      saveMessage.value = '';
    }, 3000);
  } catch (e) {
    saveMessage.value = 'Error saving settings.';
  }
};

const connectGoogle = async () => {
  // Save settings first to ensure backend has credentials
  try {
    await saveSettingsInternal();
    // Redirect to login
    window.location.href = '/api/auth/google/login';
  } catch (e) {
    alert('Failed to save settings before connecting: ' + e);
  }
};
</script>

<style scoped>
.status-success {
  color: #059669;
  background-color: #d1fae5;
  padding: 10px;
  border-radius: 4px;
  margin-top: 10px;
  width: 100%;
}

.status-error {
  color: #dc2626;
  background-color: #fee2e2;
  padding: 10px;
  border-radius: 4px;
  margin-top: 10px;
  width: 100%;
}
</style>
