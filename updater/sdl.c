#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/ioctl.h>
#include <linux/fb.h>
#include <SDL.h>

const int UPDATER_RENDER_WIDTH = 640;
const int UPDATER_RENDER_HEIGHT = 480;

static int updater_display_width = 640;
static int updater_display_height = 480;

static SDL_Window *updater_window = NULL;
static SDL_Renderer *updater_renderer = NULL;
static SDL_Texture *updater_texture = NULL;

int updater_init() {
    if (SDL_Init(SDL_INIT_VIDEO) < 0) {
        fprintf(stderr, "SDL_Init failed: %s\n", SDL_GetError());
        return -1;
    }

    // Detect display resolution from framebuffer
    int fb_fd = open("/dev/fb0", O_RDONLY);
    if (fb_fd >= 0) {
        struct fb_var_screeninfo vinfo;
        if (ioctl(fb_fd, FBIOGET_VSCREENINFO, &vinfo) == 0) {
            updater_display_width = vinfo.xres;
            updater_display_height = vinfo.yres;
        }
        close(fb_fd);
    }

    updater_window = SDL_CreateWindow("MiyooPod Updater",
        SDL_WINDOWPOS_UNDEFINED, SDL_WINDOWPOS_UNDEFINED,
        updater_display_width, updater_display_height, SDL_WINDOW_SHOWN);
    if (!updater_window) {
        fprintf(stderr, "SDL_CreateWindow failed: %s\n", SDL_GetError());
        return -1;
    }

    updater_renderer = SDL_CreateRenderer(updater_window, -1, SDL_RENDERER_ACCELERATED);
    if (!updater_renderer) {
        fprintf(stderr, "SDL_CreateRenderer failed: %s\n", SDL_GetError());
        return -1;
    }

    updater_texture = SDL_CreateTexture(updater_renderer,
        SDL_PIXELFORMAT_ABGR8888, SDL_TEXTUREACCESS_STREAMING,
        UPDATER_RENDER_WIDTH, UPDATER_RENDER_HEIGHT);
    if (!updater_texture) {
        fprintf(stderr, "SDL_CreateTexture failed: %s\n", SDL_GetError());
        return -1;
    }

    return 0;
}

int updater_refresh(unsigned char *pixels) {
    if (!updater_texture) return -1;
    SDL_UpdateTexture(updater_texture, NULL, pixels, UPDATER_RENDER_WIDTH * 4);
    SDL_RenderClear(updater_renderer);
    SDL_RenderCopy(updater_renderer, updater_texture, NULL, NULL);
    SDL_RenderPresent(updater_renderer);
    return 0;
}

int updater_poll_event() {
    SDL_Event event;
    while (SDL_PollEvent(&event)) {
        if (event.type == SDL_KEYDOWN) {
            return event.key.keysym.sym;
        }
    }
    return -1;
}

void updater_quit() {
    if (updater_texture) SDL_DestroyTexture(updater_texture);
    if (updater_renderer) SDL_DestroyRenderer(updater_renderer);
    if (updater_window) SDL_DestroyWindow(updater_window);
    SDL_Quit();
}
