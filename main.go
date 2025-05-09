// Package keyboard provides utilities for listening to keyboard events on Linux systems
// using the evdev interface. It allows registering key combinations and
// handling press, release, and hold events with callback support.
package keyboard

import (
	"fmt"
	"strings"
	"sync"

	"github.com/holoplot/go-evdev"
	"golang.org/x/exp/slices"
)

// EventType represents the type of a keyboard event: press, release, or hold.
type EventType int

const (
	// Release indicates that a key was released.
	Release EventType = iota
	// Press indicates that a key was pressed.
	Press
	// Hold indicates that a key is being held down.
	Hold
)

// String returns the string representation of the EventType.
func (e EventType) String() string {
	switch e {
	case Press:
		return "Press"
	case Release:
		return "Release"
	case Hold:
		return "Hold"
	default:
		return "Unknown"
	}
}

// Event describes a keyboard event with a key code and its EventType.
type Event struct {
	// Key is the evdev code name for the key (e.g., "KEY_A").
	Key string
	// Type is the type of event: Press, Release, or Hold.
	Type EventType
}

// findFirstKeyboard scans available evdev devices and returns the path
// of the first device that supports keyboard events.
func findFirstKeyboard() (string, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return "", fmt.Errorf("listing devices: %w", err)
	}
	for _, p := range paths {
		dev, err := evdev.Open(p.Path)
		if err != nil {
			continue
		}
		types := dev.CapableTypes()
		has := func(t evdev.EvType) bool {
			return slices.Contains(types, t)
		}
		// require key events and repeat events
		if !has(evdev.EV_KEY) || !has(evdev.EV_REP) {
			dev.Close()
			continue
		}
		// check device name for "keyboard"
		name, err := dev.Name()
		if err != nil || !strings.Contains(strings.ToLower(name), "keyboard") {
			dev.Close()
			continue
		}
		dev.Close()
		return p.Path, nil
	}
	return "", fmt.Errorf("no keyboard found")
}

// Listen opens the first detected keyboard device and returns a channel
// streaming keyboard events. It spawns a goroutine to read events until an error occurs.
func Listen() (<-chan Event, error) {
	path, err := findFirstKeyboard()
	if err != nil {
		return nil, err
	}
	dev, err := evdev.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	out := make(chan Event)
	go func() {
		defer close(out)
		defer dev.Close()
		for {
			ev, err := dev.ReadOne()
			if err != nil {
				return
			}
			if ev.Type != evdev.EV_KEY {
				continue
			}
			var et EventType
			switch ev.Value {
			case 0:
				et = Release
			case 1:
				et = Press
			case 2:
				et = Hold
			default:
				continue
			}
			out <- Event{Key: ev.CodeName(), Type: et}
		}
	}()
	return out, nil
}

// BindingCallback is the function signature for key combination callbacks.
type BindingCallback func()

// Manager handles registration of key combination bindings and dispatching
// callbacks on matching keyboard events.
type Manager struct {
	bindings        map[string]BindingCallback // registered key combos to callbacks
	pressed         map[string]bool            // currently pressed keys
	fired           map[string]bool            // combos already fired when suppressRepeats is enabled
	suppressRepeats bool                       // if true, suppress repeated events
	mu              sync.Mutex                 // protects internal state
}

// NewManager creates and returns a pointer to an initialized Manager.
func NewManager() *Manager {
	return &Manager{
		bindings: make(map[string]BindingCallback),
		pressed:  make(map[string]bool),
		fired:    make(map[string]bool),
	}
}

// SuppressRepeats enables suppression of repeated callback invocations
// until keys are released.
func (m *Manager) SuppressRepeats() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.suppressRepeats = true
}

// RegisterBinding registers a callback for a key combination specified
// by combo (e.g., "CTRL+ALT+T", "META+L").
func (m *Manager) RegisterBinding(combo string, cb BindingCallback) {
	norm := normalizeCombo(combo)
	m.mu.Lock()
	m.bindings[norm] = cb
	m.mu.Unlock()
}

func normalizeCombo(c string) string {
	parts := strings.Split(strings.ToUpper(c), "+")
	return strings.Join(parts, "+")
}

// HandleEvent processes a single Event, updates internal key state,
// and invokes any registered callbacks matching the active combination.
func (m *Manager) HandleEvent(ev Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := ev.Key
	mod := isModifier(key)

	// update pressed keys
	if ev.Type == Press {
		m.pressed[key] = true
	} else if ev.Type == Release {
		delete(m.pressed, key)
		if m.suppressRepeats {
			suffix := keyName(key)
			for combo := range m.fired {
				parts := strings.Split(combo, "+")
				if parts[len(parts)-1] == suffix {
					delete(m.fired, combo)
				}
			}
		}
	}

	// on press of non-modifier, build combo and maybe fire callback
	if ev.Type == Press && !mod {
		comboParts := []string{}
		for k := range m.pressed {
			if isModifier(k) {
				comboParts = append(comboParts, modifierName(k))
			}
		}
		comboParts = append(comboParts, keyName(key))
		combo := strings.Join(comboParts, "+")

		if m.suppressRepeats {
			if m.fired[combo] {
				return
			}
			m.fired[combo] = true
		}

		if cb, ok := m.bindings[combo]; ok {
			go cb()
		}
	}
}

// isModifier returns true if the given key code is a modifier key.
func isModifier(key string) bool {
	switch key {
	case "KEY_LEFTCTRL", "KEY_RIGHTCTRL",
		"KEY_LEFTSHIFT", "KEY_RIGHTSHIFT",
		"KEY_LEFTALT", "KEY_RIGHTALT",
		"KEY_LEFTMETA", "KEY_RIGHTMETA":
		return true
	}
	return false
}

// modifierName converts a modifier key code (e.g., "KEY_LEFTCTRL") to its
// human-readable name (e.g., "CTRL").
func modifierName(key string) string {
	key = strings.TrimPrefix(key, "KEY_")
	key = strings.TrimPrefix(key, "LEFT")
	key = strings.TrimPrefix(key, "RIGHT")
	return key
}

// keyName returns the human-readable portion of a key code by trimming
// the "KEY_" prefix.
func keyName(key string) string {
	return strings.TrimPrefix(key, "KEY_")
}
