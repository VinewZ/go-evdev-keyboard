# go-evdev-keyboard

`go-evdev-keyboard` is a Go library for listening to keyboard events on Linux systems via the `evdev` interface.
It provides easy-to-use utilities for detecting key presses, releases, and holds, and for binding custom callbacks to key combinations.

## Dependencies
[github.com/holoplot/go-evdev](https://github.com/holoplot/go-evdev)

## Features

* Automatically detect the first available keyboard device
* Register callbacks for arbitrary key combinations (e.g., `CTRL+ALT+T`)
* Optional suppression of repeated key events

## Installation

```bash
go get github.com/VinewZ/go-evdev-keyboard@v1.0.0
```

## Usage

```go
package main

import (
  "context"
  "log"

  "github.com/VinewZ/go-evdev-keyboard"
)

func main() {
  // Create a cancellable context
  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

  // Setup keyboard manager
  mgr := keyboard.NewManager()

  // Optional: suppress repeated firing while a key is held
  mgr.SuppressRepeats()

  // Register a key combination callback
  mgr.RegisterBinding("CTRL+ALT+T", func() {
    log.Println("CTRL+ALT+T pressed!")
  })

  // Register a key combination callback
  mgr.RegisterBinding("META+O", func() {
    log.Println("META+O pressed!")
  })

  // Start listening for keyboard events
  go func() {
    events, err := keyboard.Listen()
    if err != nil {
      log.Fatalf("cannot listen: %v", err)
    }
    for {
      select {
      case <-ctx.Done():
        return
      case ev, ok := <-events:
        if !ok {
          return
        }
        mgr.HandleEvent(ev)
      }
    }
  }()

  // Block until context is canceled
  // Can be replaced by any other blocking operation
  <-ctx.Done()
}
```
