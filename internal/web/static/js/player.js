(function () {
  'use strict';

  if (typeof shaka === 'undefined') return;

  document.addEventListener('alpine:init', () => {
    Alpine.data('player', () => ({
    // --- playback state ---
    playing: false,
    currentTime: 0,
    duration: 0,
    volume: 1,
    muted: false,
    playbackRate: 1,
    ended: false,
    buffering: true,

    // --- fullscreen ---
    fullscreen: false,

    // --- tracks ---
    videoTracks: [],
    audioTracks: [],
    textTracks: [],
    activeVideoTrack: null,
    activeAudioTrack: null,
    activeTextTrack: null,
    subtitlesVisible: false,

    // --- chapters ---
    chapters: [],

    // --- ui ---
    controlsVisible: true,
    controlsTimer: null,
    showChapters: false,
    showAudio: false,
    showSubtitles: false,
    showShortcuts: false,

    // --- internals ---
    player_: null,
    containerEl: null,
    hideTimeout_: null,

    // ================================================================
    // Init
    // ================================================================

    init() {
      this.containerEl = this.$el;
      const video = this.$refs.video;
      const streamUrl = this.$el.dataset.streamUrl;
      if (!streamUrl) return;

      shaka.polyfill.installAll();
      if (!shaka.Player.isBrowserSupported()) {
        this.showError('Your browser does not support this video player.');
        return;
      }

      const player = new shaka.Player();
      player.__v_skip = true;
      this.player_ = player;

      player.addEventListener('error', (e) => this.onShakaError(e));
      player.addEventListener('loading', () => { this.buffering = true; });
      player.addEventListener('loaded', () => this.onLoaded());
      player.addEventListener('trackschanged', () => this.refreshTracks());
      player.addEventListener('adaptation', () => this.onAdaptation());
      player.addEventListener('buffering', (e) => { this.buffering = e.buffering; });
      player.addEventListener('unloading', () => this.resetTracks());

      player.attach(video, true).then(() => {
        const isMp4 = streamUrl.match(/\.mp4($|\?)/i);
        if (isMp4) {
          video.src = streamUrl;
          video.addEventListener('loadedmetadata', () => {
            this.buffering = false;
            this.duration = video.duration;
            this.refreshTracks();
          });
        } else {
          player.load(streamUrl).then(() => {
            video.play().catch(function () {});
          });
        }
      }).catch((err) => this.onShakaError(err));

      this.setupVideoEvents(video);
      this.setupKeyboardShortcuts();
    },

    // ================================================================
    // Video element events
    // ================================================================

    setupVideoEvents(video) {
      const events = {
        timeupdate: () => { this.currentTime = video.currentTime; },
        play: () => { this.playing = true; },
        pause: () => { this.playing = false; },
        volumechange: () => { this.volume = video.volume; this.muted = video.muted; },
        ended: () => { this.ended = true; this.playing = false; },
        durationchange: () => { this.duration = video.duration; },
        ratechange: () => { this.playbackRate = video.playbackRate; },
        waiting: () => { this.buffering = true; },
        canplay: () => { this.buffering = false; },
        loadedmetadata: () => { this.duration = video.duration; },
      };
      for (const [ev, fn] of Object.entries(events)) {
        video.addEventListener(ev, fn);
      }
    },

    // ================================================================
    // Shaka events
    // ================================================================

    onLoaded() {
      this.buffering = false;
      this.duration = this.$refs.video.duration;
      this.refreshTracks();
      this.refreshChapters();
    },

    onAdaptation() {
      if (this.player_) {
        const tracks = this.player_.getVariantTracks();
        const active = tracks.find(function (t) { return t.active; });
        if (active) {
          this.activeVideoTrack = this.videoTracks.find(function (t) { return t.id === active.id; }) || null;
        }
      }
    },

    onShakaError(e) {
      const err = e.detail || e;
      if (err instanceof shaka.util.Error && err.severity === shaka.util.Error.Severity.CRITICAL) {
        console.error('Shaka Player fatal error:', err);
        this.buffering = false;
        this.showError('Playback error: ' + (err.message || err.code || 'unknown'));
        return;
      }
      console.warn('Shaka Player (recoverable):', err);
    },

    showError(msg) {
      this.$refs.errorMessage.textContent = msg;
      this.$refs.errorOverlay.classList.remove('hidden');
    },

    // ================================================================
    // Tracks
    // ================================================================

    refreshTracks() {
      if (!this.player_) return;
      try {
        this.videoTracks = this.player_.getVideoTracks() || [];
        this.audioTracks = this.player_.getAudioTracks() || [];
        this.textTracks = this.player_.getTextTracks() || [];
      } catch (_) {}

      this.activeVideoTrack = this.videoTracks.find(function (t) { return t.active; }) || null;
      this.activeAudioTrack = this.audioTracks.find(function (t) { return t.active; }) || null;
      this.activeTextTrack = this.textTracks.find(function (t) { return t.active; }) || null;
      this.subtitlesVisible = this.player_.isTextVisible();
    },

    resetTracks() {
      this.videoTracks = [];
      this.audioTracks = [];
      this.textTracks = [];
      this.activeVideoTrack = null;
      this.activeAudioTrack = null;
      this.activeTextTrack = null;
      this.chapters = [];
    },

    selectVideoTrack(track) {
      if (!this.player_) return;
      this.player_.selectVideoTrack(track);
      this.activeVideoTrack = track;
    },

    selectAudioTrack(track) {
      if (!this.player_) return;
      this.player_.selectAudioTrack(track);
      this.activeAudioTrack = track;
      this.showAudio = false;
    },

    selectTextTrack(track) {
      if (!this.player_) return;
      if (track) {
        this.player_.selectTextTrack(track);
        this.activeTextTrack = track;
        // this.player_.setTextVisibility(true);
        this.subtitlesVisible = true;
      } else {
        // this.player_.setTextVisibility(false);
        this.player_.selectTextTrack(null);
        this.activeTextTrack = null;
        this.subtitlesVisible = false;
      }
      this.showSubtitles = false;
    },

    toggleSubtitles() {
      if (this.textTracks.length === 0) return;
      if (this.subtitlesVisible) {
        this.subtitlesVisible = false;
      } else {
        this.subtitlesVisible = true;
      }

      if (this.subtitlesVisible && !this.activeTextTrack && this.textTracks.length > 0) {
        this.selectTextTrack(this.textTracks[0]);
      } else {
        this.selectTextTrack(null);
      }
    },

    // ================================================================
    // Chapters
    // ================================================================

    refreshChapters() {
      if (!this.player_) return;
      try {
        const regions = this.player_.getAllTimelineRegions() || [];
        const ch = regions.filter(function (r) { return r.startTime >= 0; });
        ch.sort(function (a, b) { return a.startTime - b.startTime; });
        this.chapters = ch;
      } catch (_) {
        this.chapters = [];
      }
    },

    seekToChapter(chapter) {
      this.$refs.video.currentTime = chapter.startTime;
      this.showChapters = false;
    },

    // ================================================================
    // Controls
    // ================================================================

    togglePlay() {
      const v = this.$refs.video;
      if (v.paused) { v.play(); } else { v.pause(); }
    },

    seek(offset) {
      this.$refs.video.currentTime = Math.max(0, Math.min(this.duration, this.$refs.video.currentTime + offset));
    },

    seekToFraction(fraction) {
      this.$refs.video.currentTime = fraction * this.duration;
    },

    setVolume(val) {
      const v = Math.max(0, Math.min(1, val));
      this.$refs.video.volume = v;
      if (v > 0) this.$refs.video.muted = false;
    },

    toggleMute() {
      this.$refs.video.muted = !this.$refs.video.muted;
    },

    toggleFullscreen() {
      if (!document.fullscreenElement) {
        this.containerEl.requestFullscreen().then(function () {}).catch(function () {});
      } else {
        document.exitFullscreen().then(function () {}).catch(function () {});
      }
    },

    onFullscreenChange() {
      this.fullscreen = !!document.fullscreenElement;
    },

    setPlaybackRate(rate) {
      this.$refs.video.playbackRate = rate;
    },

    // ================================================================
    // UI helpers
    // ================================================================

    showControls() {
      this.controlsVisible = true;
      clearTimeout(this.hideTimeout_);
      this.hideTimeout_ = setTimeout(() => {
        if (this.playing) this.controlsVisible = false;
      }, 3000);
    },

    hideControls() {
      if (this.playing) {
        this.controlsVisible = false;
        clearTimeout(this.hideTimeout_);
      }
    },

    closeAllMenus() {
      this.showChapters = false;
      this.showAudio = false;
      this.showSubtitles = false;
      this.showShortcuts = false;
    },

    toggleChaptersMenu() {
      this.closeAllMenus();
      this.showChapters = !this.showChapters;
    },

    toggleAudioMenu() {
      this.closeAllMenus();
      this.showAudio = !this.showAudio;
    },

    toggleSubtitlesMenu() {
      this.closeAllMenus();
      this.showSubtitles = !this.showSubtitles;
    },

    formatTime(t) {
      if (!t || t < 0) return '0:00';
      const h = Math.floor(t / 3600);
      const m = Math.floor((t % 3600) / 60);
      const s = Math.floor(t % 60);
      const ss = s < 10 ? '0' + s : '' + s;
      if (h > 0) {
        const mm = m < 10 ? '0' + m : '' + m;
        return h + ':' + mm + ':' + ss;
      }
      return m + ':' + ss;
    },

    formatSeconds(seconds) {
      const m = Math.floor(seconds / 60);
      const s = Math.floor(seconds % 60);
      return m + ':' + (s < 10 ? '0' : '') + s;
    },

    // ================================================================
    // Keyboard shortcuts
    // ================================================================

    setupKeyboardShortcuts() {
      document.addEventListener('keydown', (e) => this.onKeyDown(e));
      document.addEventListener('fullscreenchange', () => this.onFullscreenChange());
    },

    onKeyDown(e) {
      const tag = e.target.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

      const key = e.key;
      const ctrl = e.ctrlKey || e.metaKey;

      // Close dropdowns on Escape
      if (key === 'Escape') {
        if (this.showShortcuts) { this.showShortcuts = false; e.preventDefault(); return; }
        if (this.showChapters || this.showAudio || this.showSubtitles) {
          this.closeAllMenus();
          e.preventDefault();
          return;
        }
        if (this.fullscreen) {
          document.exitFullscreen();
          e.preventDefault();
          return;
        }
      }

      if (ctrl) return; // ignore ctrl combinations

      switch (key) {
        case ' ':
        case 'k':
        case 'K':
          e.preventDefault();
          this.togglePlay();
          break;
        case 'f':
        case 'F':
          e.preventDefault();
          this.toggleFullscreen();
          break;
        case 'Enter':
          if (this.fullscreen) break; // don't fullscreen when already fullscreen
          break;
        case 'm':
        case 'M':
          e.preventDefault();
          this.toggleMute();
          break;
        case 'ArrowLeft':
          e.preventDefault();
          this.seek(-5);
          break;
        case 'ArrowRight':
          e.preventDefault();
          this.seek(5);
          break;
        case 'j':
        case 'J':
          e.preventDefault();
          this.seek(-10);
          break;
        case 'l':
        case 'L':
          e.preventDefault();
          this.seek(10);
          break;
        case 'ArrowUp':
          e.preventDefault();
          this.setVolume(this.volume + 0.1);
          break;
        case 'ArrowDown':
          e.preventDefault();
          this.setVolume(this.volume - 0.1);
          break;
        case 'c':
        case 'C':
          e.preventDefault();
          this.toggleSubtitles();
          break;
        case ',':
        case '<':
          e.preventDefault();
          this.setPlaybackRate(Math.max(0.25, this.playbackRate - 0.25));
          break;
        case '.':
        case '>':
          e.preventDefault();
          this.setPlaybackRate(Math.min(16, this.playbackRate + 0.25));
          break;
        case 'Home':
          e.preventDefault();
          this.seekToFraction(0);
          break;
        case 'End':
          e.preventDefault();
          this.seekToFraction(1);
          break;
        case '?':
          e.preventDefault();
          this.showShortcuts = !this.showShortcuts;
          break;
        default:
          if (key >= '0' && key <= '9') {
            e.preventDefault();
            this.seekToFraction(parseInt(key, 10) / 10);
          }
          break;
      }
    },

    // ================================================================
    // Seek bar helpers
    // ================================================================

    seekBarStyle() {
      if (!this.duration) return { width: '0%' };
      return { width: (this.currentTime / this.duration * 100) + '%' };
    },

    chapterStyle(ch) {
      if (!this.duration) return { left: '0%' };
      return { left: (ch.startTime / this.duration * 100) + '%' };
    },

    videoLabel(track) {
      if (!track) return 'Auto';
      var label = track.label || '';
      var lang = track.language || '';
      var h = track.height || 0;
      var bw = track.bandwidth || 0;
      if (h) {
        label = label || (h + 'p');
        if (bw) label += ' (' + Math.round(bw / 1000) + ' kbps)';
      }
      return label || lang || 'Unknown';
    },

    audioLabel(track) {
      if (!track) return 'Unknown';
      var label = track.label || '';
      var lang = track.language || '';
      return label || lang || 'Unknown';
    },

    textLabel(track) {
      if (!track) return 'Off';
      var label = track.label || '';
      var lang = track.language || '';
      return label || lang || 'Unknown';
    },
  }));
  });
})();
