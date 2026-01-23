<template>
  <div class="container">
    <header>
      <div class="header-title">
        <!-- SVG Icon inline or placeholder -->
        <h1>PhotoFrame Server</h1>
      </div>
    </header>

    <Gallery />
    <Settings />

    <footer>
      <p>ESP32-S3 PhotoFrame Server</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import Settings from './components/Settings.vue';
import Gallery from './components/Gallery.vue';
</script>
