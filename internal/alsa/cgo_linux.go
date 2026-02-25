//go:build linux && cgo

package alsa

/*
#cgo LDFLAGS: -lasound
#include <stdlib.h>
#include <alsa/asoundlib.h>

// Helper to get sorted control names
// Returns an array of strings
// Caller must free the array using free_names
static char** get_sorted_controls(int card_index, int* count) {
    snd_mixer_t *handle;
    char card[64];

    sprintf(card, "hw:%d", card_index);

    if (snd_mixer_open(&handle, 0) < 0) return NULL;
    if (snd_mixer_attach(handle, card) < 0) {
        snd_mixer_close(handle);
        return NULL;
    }
    if (snd_mixer_selem_register(handle, NULL, NULL) < 0) {
        snd_mixer_close(handle);
        return NULL;
    }
    if (snd_mixer_load(handle) < 0) {
        snd_mixer_close(handle);
        return NULL;
    }

    // Count elements
    int n = 0;
    snd_mixer_elem_t* elem;
    for (elem = snd_mixer_first_elem(handle); elem; elem = snd_mixer_elem_next(elem)) {
        n++;
    }

    if (n == 0) {
        snd_mixer_close(handle);
        *count = 0;
        return NULL;
    }

    char** names = (char**)calloc(n, sizeof(char*));
    int i = 0;

    for (elem = snd_mixer_first_elem(handle); elem; elem = snd_mixer_elem_next(elem)) {
        const char* name = snd_mixer_selem_get_name(elem);
        names[i++] = strdup(name);
    }

    snd_mixer_close(handle);
    *count = n;
    return names;
}

static void free_names(char** names, int count) {
    if (names == NULL) return;
    int i;
    for (i = 0; i < count; i++) {
        free(names[i]);
    }
    free(names);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// getControlNamesInOrder returns control names in the order alsamixer uses.
// It uses CGo to access libasound directly.
func (m *Mixer) getControlNamesInOrder(card uint) ([]string, error) {
	var count C.int
	cNames := C.get_sorted_controls(C.int(card), &count)
	if cNames == nil {
		if count == 0 {
			return nil, fmt.Errorf("no controls found for card %d", card)
		}
		return nil, fmt.Errorf("failed to open mixer for card %d", card)
	}
	defer C.free_names(cNames, count)

	// Convert C array of strings to Go slice
	length := int(count)
	// Create a slice backed by the C array to iterate easily
	// This is a common trick to turn a C pointer into a Go slice
	// unsafe.Slice requires Go 1.17+
	slice := unsafe.Slice(cNames, length)

	names := make([]string, length)
	for i, cStr := range slice {
		names[i] = C.GoString(cStr)
	}

	return names, nil
}
