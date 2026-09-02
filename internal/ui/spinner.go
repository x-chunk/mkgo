package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// spinnerFrames is the braille animation used on interactive terminals.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// asciiSpinnerFrames keeps the animation alive when emoji and other non-ASCII
// glyphs are disabled.
var asciiSpinnerFrames = []string{"|", "/", "-", "\\"}

const spinnerInterval = 90 * time.Millisecond

// spinner animates a single line of text in place. It is only started on
// interactive terminals; elsewhere the Printer falls back to static lines.
type spinner struct {
	out      io.Writer
	frames   []string
	style    Style
	interval time.Duration

	mu      sync.Mutex
	message string
	done    chan struct{}
	wg      sync.WaitGroup
	running bool
}

func newSpinner(out io.Writer, frames []string, style Style) *spinner {
	return &spinner{out: out, frames: frames, style: style, interval: spinnerInterval}
}

// start begins the animation with an initial message.
func (s *spinner) start(message string) {
	s.mu.Lock()
	if s.running {
		s.message = message
		s.mu.Unlock()
		return
	}
	s.running = true
	s.message = message
	s.done = make(chan struct{})
	done := s.done
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.render(i)
			}
		}
	}()
	s.render(0)
}

// update swaps the message without restarting the animation.
func (s *spinner) update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// stop halts the animation and erases the spinner line.
func (s *spinner) stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.done)
	s.mu.Unlock()

	s.wg.Wait()
	s.clear()
}

func (s *spinner) render(i int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	frame := s.frames[i%len(s.frames)]
	fmt.Fprintf(s.out, "\r\033[2K%s %s", s.style.Apply(frame), s.message)
}

func (s *spinner) clear() {
	fmt.Fprint(s.out, "\r\033[2K")
}

// active reports whether the animation goroutine is currently running.
func (s *spinner) active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
