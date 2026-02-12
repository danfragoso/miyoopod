#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/ioctl.h>
#include <linux/fb.h>
#include <SDL.h>

// Forward declarations for Go functions
extern void GoLogMsg(char* msg);
extern void DetectDevice(int width, int height);

const int RENDER_WIDTH = 640;
const int RENDER_HEIGHT = 480;

static int display_width = 640;
static int display_height = 480;

static SDL_Window *window = NULL;
static SDL_Renderer *renderer = NULL;
static SDL_Texture *texture = NULL;

void c_log(const char *msg) {
    char buffer[512];
    snprintf(buffer, sizeof(buffer), "[C] INFO: %s", msg);
    GoLogMsg(buffer);
}

void c_logf(const char *fmt, const char *detail) {
    char buffer[512];
    snprintf(buffer, sizeof(buffer), "[C] INFO: ");
    int offset = strlen(buffer);
    snprintf(buffer + offset, sizeof(buffer) - offset, fmt, detail);
    GoLogMsg(buffer);
}

void c_logd(const char *fmt, int val) {
    char buffer[512];
    snprintf(buffer, sizeof(buffer), "[C] INFO: ");
    int offset = strlen(buffer);
    snprintf(buffer + offset, sizeof(buffer) - offset, fmt, val);
    GoLogMsg(buffer);
}

int pollEvents() {
    SDL_Event event;
    while (SDL_PollEvent(&event)) {
        if (event.type == SDL_KEYDOWN && event.key.repeat == 0) {
            return event.key.keysym.sym;
        }
        // Return negative value for key release (to distinguish from key press)
        // We use -(keycode + 1) to avoid collision with -1 (no event)
        if (event.type == SDL_KEYUP) {
            return -(event.key.keysym.sym + 1);
        }
    }
    return -1;
}

int refreshScreenPtr(unsigned char *pixels) {
    if (!texture) return -1;

    SDL_UpdateTexture(texture, NULL, pixels, RENDER_WIDTH * 4);
    SDL_RenderClear(renderer);
    // Scale the 640x480 texture to fit the actual display resolution
    SDL_RenderCopy(renderer, texture, NULL, NULL);
    SDL_RenderPresent(renderer);
    return 0;
}


int init() {
    c_log("SDL2 init (VIDEO | AUDIO)...");
    if (SDL_Init(SDL_INIT_VIDEO | SDL_INIT_AUDIO) < 0) {
        c_logf("SDL_Init failed: %s", SDL_GetError());
        return -1;
    }
    c_log("SDL_Init OK");

    // Detect display resolution from framebuffer device
    int fb_fd = open("/dev/fb0", O_RDONLY);
    if (fb_fd >= 0) {
        struct fb_var_screeninfo vinfo;
        if (ioctl(fb_fd, FBIOGET_VSCREENINFO, &vinfo) == 0) {
            display_width = vinfo.xres;
            display_height = vinfo.yres;
            c_logd("Detected FB resolution width: %d", display_width);
            c_logd("Detected FB resolution height: %d", display_height);
            
            // Notify Go about detected device
            DetectDevice(display_width, display_height);
        } else {
            c_log("Could not get FB info, using default 640x480");
            display_width = 640;
            display_height = 480;
            DetectDevice(display_width, display_height);
        }
        close(fb_fd);
    } else {
        c_log("Could not open /dev/fb0, using default 640x480");
        display_width = 640;
        display_height = 480;
        DetectDevice(display_width, display_height);
    }

    c_log("Creating window...");
    window = SDL_CreateWindow("MiyooPod",
        SDL_WINDOWPOS_UNDEFINED, SDL_WINDOWPOS_UNDEFINED,
        display_width, display_height, SDL_WINDOW_SHOWN);
    if (!window) {
        c_logf("SDL_CreateWindow failed: %s", SDL_GetError());
        return -1;
    }
    c_log("Window created");

    c_log("Creating renderer...");
    renderer = SDL_CreateRenderer(window, -1, SDL_RENDERER_ACCELERATED);
    if (!renderer) {
        c_logf("SDL_CreateRenderer failed: %s", SDL_GetError());
        return -1;
    }
    c_log("Renderer created");

    c_log("Creating texture at 640x480 (ABGR8888)...");
    texture = SDL_CreateTexture(renderer,
        SDL_PIXELFORMAT_ABGR8888, SDL_TEXTUREACCESS_STREAMING,
        RENDER_WIDTH, RENDER_HEIGHT);
    if (!texture) {
        c_logf("SDL_CreateTexture failed: %s", SDL_GetError());
        return -1;
    }
    c_log("Texture created");

    return 0;
}

void quit() {
    if (texture) {
        SDL_DestroyTexture(texture);
    }
    if (renderer) {
        SDL_DestroyRenderer(renderer);
    }
    if (window) {
        SDL_DestroyWindow(window);
    }
    SDL_Quit();
}
