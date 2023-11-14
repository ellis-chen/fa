package atexit

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

var (
	// +checklocks:fnsMutex
	fns          = make(map[FnHandle]bool)
	fnsMutex     sync.Mutex
	exitChan     chan os.Signal
	exitOnce     sync.Once
	registerOnce sync.Once
	signalled    int32
)

// FnHandle is the type of the handle returned by function `Register`
// that can be used to unregister an at-exit function
type FnHandle *func()

// Register a function to be called on exit.
// Returns a handle which can be used to unregister the function with `Unregister`.
func Register(fn func()) FnHandle {
	fnsMutex.Lock()
	fns[&fn] = true
	fnsMutex.Unlock()

	// Run AtExit handlers on exitSignals so everything gets tidied up properly
	registerOnce.Do(func() {
		exitChan = make(chan os.Signal, 1)
		signal.Notify(exitChan, exitSignals...)
		go func() {
			sig := <-exitChan
			if sig == nil {
				return
			}
			atomic.StoreInt32(&signalled, 1)
			logrus.Infof("Signal received: %s", sig)
			Run()
			logrus.Infof("Exiting...")
			os.Exit(0)
		}()
	})

	return &fn
}

// Signalled returns true if an exit signal has been received
func Signalled() bool {
	return atomic.LoadInt32(&signalled) != 0
}

// Unregister a function using the handle returned by `Register`
func Unregister(handle FnHandle) {
	fnsMutex.Lock()
	defer fnsMutex.Unlock()
	delete(fns, handle)
}

// IgnoreSignals disables the signal handler and prevents Run from being executed automatically
func IgnoreSignals() {
	registerOnce.Do(func() {})
	if exitChan != nil {
		signal.Stop(exitChan)
		close(exitChan)
		exitChan = nil
	}
}

// Run all the at exit functions if they haven't been run already
func Run() {
	exitOnce.Do(func() {
		fnsMutex.Lock()
		defer fnsMutex.Unlock()
		for fnHandle := range fns {
			(*fnHandle)()
		}
	})
}

// OnError registers fn with atexit and returns a function which
// runs fn() if *perr != nil and deregisters fn
//
// It should be used in a defer statement normally so
//
//	defer OnError(&err, cancelFunc)()
//
// So cancelFunc will be run if the function exits with an error or
// at exit.
func OnError(perr *error, fn func()) func() {
	handle := Register(fn)

	return func() {
		defer Unregister(handle)
		if *perr != nil {
			fn()
		}
	}

}
