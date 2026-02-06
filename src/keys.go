package main

type Key int

// SDL2 keycodes as mapped by the steward-fu Miyoo Mini SDL2 driver
// (SDL_event_mini.c maps Linux input keycodes → SDL keycodes)
const (
	NONE Key = -1

	// D-pad (SDLK_UP/DOWN/LEFT/RIGHT = SDL_SCANCODE | 1<<30)
	UP    Key = 1073741906 // SDLK_UP
	DOWN  Key = 1073741905 // SDLK_DOWN
	LEFT  Key = 1073741904 // SDLK_LEFT
	RIGHT Key = 1073741903 // SDLK_RIGHT

	// Face buttons
	A Key = 32         // SDLK_SPACE  (KEY_SPACE)
	B Key = 1073742048 // SDLK_LCTRL (KEY_LEFTCTRL)
	X Key = 1073742049 // SDLK_LSHIFT (KEY_LEFTSHIFT)
	Y Key = 1073742050 // SDLK_LALT (KEY_LEFTALT)

	// Shoulder buttons
	L  Key = 101        // SDLK_e (KEY_E)
	L2 Key = 9          // SDLK_TAB (KEY_TAB)
	R  Key = 116        // SDLK_t (KEY_T)
	R2 Key = 8          // SDLK_BACKSPACE (KEY_BACKSPACE)

	// System buttons
	SELECT Key = 1073742052 // SDLK_RCTRL (KEY_RIGHTCTRL)
	START  Key = 13         // SDLK_RETURN (KEY_ENTER)
	MENU   Key = 27         // SDLK_ESCAPE (KEY_ESC)
)

