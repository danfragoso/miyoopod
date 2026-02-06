#include "SDL.h"
#include "SDL_mixer.h"
#include <stdlib.h>
#include <string.h>

static Mix_Music *current_music = NULL;
static volatile int music_finished_flag = 0;
static double cached_duration = 0.0;

// Memory buffer for current track - eliminates SD card I/O during playback
static void *current_music_data = NULL;

static void on_music_finished() {
    music_finished_flag = 1;
}

int audio_init() {
    c_log("audio_init entered");

    c_log("calling Mix_OpenAudio...");
    if (Mix_OpenAudio(44100, MIX_DEFAULT_FORMAT, 2, 65536) < 0) {
        c_logf("Mix_OpenAudio failed: %s", SDL_GetError());
        return -1;
    }
    c_log("Mix_OpenAudio OK");

    int flags = MIX_INIT_MP3;
    int initted = Mix_Init(flags);
    if ((initted & flags) != flags) {
        c_logf("Mix_Init MP3 failed: %s", Mix_GetError());
    } else {
        c_log("Mix_Init MP3 OK");
    }

    Mix_HookMusicFinished(on_music_finished);
    return 0;
}

// Load from file path (streaming from SD card - fallback)
int audio_load(const char *path) {
    if (current_music) {
        Mix_FreeMusic(current_music);
        current_music = NULL;
    }
    if (current_music_data) {
        free(current_music_data);
        current_music_data = NULL;
    }

    current_music = Mix_LoadMUS(path);
    if (!current_music) {
        c_logf("Mix_LoadMUS failed: %s", Mix_GetError());
        return -1;
    }

    cached_duration = Mix_MusicDuration(current_music);
    return 0;
}

// Load audio from a memory buffer (data is C-allocated, caller gives ownership).
// Eliminates SD card I/O during playback - SDL_mixer reads from RAM.
// Returns 0 on success, -1 on failure. On failure, caller must NOT free data (we do).
int audio_load_mem(void *data, int size) {
    if (current_music) {
        Mix_FreeMusic(current_music);
        current_music = NULL;
    }
    if (current_music_data) {
        free(current_music_data);
        current_music_data = NULL;
    }

    current_music_data = data;

    SDL_RWops *rw = SDL_RWFromMem(current_music_data, size);
    if (!rw) {
        c_logf("SDL_RWFromMem failed: %s", SDL_GetError());
        free(current_music_data);
        current_music_data = NULL;
        return -1;
    }

    current_music = Mix_LoadMUS_RW(rw, 1);
    if (!current_music) {
        c_logf("Mix_LoadMUS_RW failed: %s", Mix_GetError());
        free(current_music_data);
        current_music_data = NULL;
        return -1;
    }

    cached_duration = Mix_MusicDuration(current_music);
    c_log("audio_load_mem OK (playing from RAM)");
    return 0;
}

int audio_play() {
    if (!current_music) return -1;
    music_finished_flag = 0;
    return Mix_PlayMusic(current_music, 0);
}

void audio_pause() {
    Mix_PauseMusic();
}

void audio_resume() {
    Mix_ResumeMusic();
}

void audio_toggle_pause() {
    if (Mix_PausedMusic()) {
        audio_resume();
    } else {
        audio_pause();
    }
}

void audio_stop() {
    Mix_HaltMusic();
    music_finished_flag = 0;
    cached_duration = 0.0;
}

int audio_is_playing() {
    return Mix_PlayingMusic() && !Mix_PausedMusic();
}

int audio_is_paused() {
    return Mix_PausedMusic();
}

double audio_get_position() {
    if (!current_music || !Mix_PlayingMusic()) return 0.0;
    return Mix_GetMusicPosition(current_music);
}

double audio_get_duration() {
    return cached_duration;
}

// Get duration of an audio file without loading it into the player
double audio_get_file_duration(const char *path) {
    Mix_Music *temp_music = Mix_LoadMUS(path);
    if (!temp_music) {
        return 0.0;
    }
    
    double duration = Mix_MusicDuration(temp_music);
    Mix_FreeMusic(temp_music);
    
    return duration;
}

int audio_seek(double position) {
    if (!current_music) return -1;
    return Mix_SetMusicPosition(position);
}

void audio_set_volume(int volume) {
    Mix_VolumeMusic(volume);
}

int audio_check_finished() {
    if (music_finished_flag) {
        music_finished_flag = 0;
        return 1;
    }
    return 0;
}

typedef struct {
    double position;
    double duration;
    int is_playing;
    int is_paused;
    int finished;
} AudioState;

void audio_get_state(AudioState *state) {
    state->position = 0.0;
    state->duration = cached_duration;
    state->is_playing = 0;
    state->is_paused = 0;
    state->finished = 0;

    if (current_music && Mix_PlayingMusic()) {
        state->position = Mix_GetMusicPosition(current_music);
        state->is_playing = !Mix_PausedMusic();
        state->is_paused = Mix_PausedMusic();
    }

    if (music_finished_flag) {
        music_finished_flag = 0;
        state->finished = 1;
    }
}

void audio_quit() {
    if (current_music) {
        Mix_FreeMusic(current_music);
        current_music = NULL;
    }
    if (current_music_data) {
        free(current_music_data);
        current_music_data = NULL;
    }
    Mix_CloseAudio();
    Mix_Quit();
}
