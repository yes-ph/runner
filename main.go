package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	logger := newLogger(ERROR)

	switch os.Getenv("LOG_LEVEL") {
	case "info":
		logger.setLevel(INFO)
	case "debug":
		logger.setLevel(DEBUG)
	}

	defer func() {
		if r := recover(); r != nil {
			logger.error("Error:", r)
			debug.PrintStack()
		}
	}()

	logger.info("Main process pid:", os.Getpid())

	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	logger.info("Directory:", root)

	binaryName := "runner"
	fullBinaryName := filepath.Join(root, binaryName)

	done := make(chan bool)
	restartWatcher := make(chan bool)
	walk := make(chan bool)
	runCommand := make(chan bool)

	var timerWatcher *time.Timer
	var timerRunner *time.Timer
	var watcher *fsnotify.Watcher
	var cmdRunner *exec.Cmd
	var debounceInterval = 500 * time.Microsecond
	var runnerDebounce = 2 * time.Second

	defer watcher.Close()

	go func() {
		for {
			if <-runCommand {
				if timerRunner != nil {
					logger.info("Replacing timer...")
					timerRunner.Stop()
				}
				timerRunner = time.AfterFunc(runnerDebounce, func() {
					defer func() {
						if r := recover(); r != nil {
							logger.error(r)
						}
					}()
					logger.debug("Building...")
					cmd := exec.Command("go", "build", "-o", binaryName, ".")
					cmd.Dir = root
					err := cmd.Run()
					if err != nil {
						panic(err)
					}
					if cmdRunner != nil && cmdRunner.Process != nil {
						pid := cmdRunner.Process.Pid
						logger.debug("Killing process id", pid)
						proc, err := os.FindProcess(pid)
						if err != nil {
							panic(err)
						}

						err = proc.Signal(syscall.SIGTERM)
						if err != nil {
							panic(err)
						}

						_, err = proc.Wait()
						if err != nil {
							panic(err)
						}

						logger.info("Process killed")
					}
					logger.debug("Running...")
					cmdRunner = exec.Command(fullBinaryName)
					cmdRunner.Dir = root
					cmdRunner.Stdout = os.Stdout
					cmdRunner.Stderr = os.Stderr
					err = cmdRunner.Start()
					if err != nil {
						panic(err)
					}
					logger.infof("Process id %v started", cmdRunner.Process.Pid)
				})
			}
		}
	}()

	go func() {
		for {
			if <-walk {
				logger.info("Walking...")
				err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						panic(err)
					}
					if info.Name() == ".git" {
						return filepath.SkipDir
					}
					if info.IsDir() {
						err = watcher.Add(path)
						if err != nil {
							panic(err)
						}
					}
					return nil
				})
				if err != nil {
					panic(err)
				}
			}
		}
	}()

	go func() {
		for {
			if <-restartWatcher {
				logger.info("Restart called")
				if watcher == nil {
					logger.info("Starting watcher...")
					watcher, err = fsnotify.NewWatcher()
					if err != nil {
						panic(err)
					}
					walk <- true
				} else {
					if timerWatcher != nil {
						logger.debug("Replacing timer...")
						timerWatcher.Stop()
					}
					timerWatcher = time.AfterFunc(debounceInterval, func() {
						logger.debug("Restarting watcher...")
						watcher, err = fsnotify.NewWatcher()
						if err != nil {
							panic(err)
						}
						walk <- true
					})
				}
			}
		}
	}()

	restartWatcher <- true
	runCommand <- true

	go func() {
		for {
			if watcher == nil {
				logger.info("Waiting for watcher...")
				time.Sleep(1 * time.Second)
				continue
			}
			select {
			case event := <-watcher.Events:
				if event.Name == fullBinaryName {
					continue
				}
				logger.info("Event:", event.Op, event.Name)
				runCommand <- true
				if event.Op&fsnotify.Write == fsnotify.Write {
					logger.info("Modified file:", event.Name)
				} else {
					restartWatcher <- true
				}
			case err := <-watcher.Errors:
				panic(err)
			}
		}
	}()

	<-done
}
