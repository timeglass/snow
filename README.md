# snow
The watcher that knows nothing.

## Introduction
There have been several attempts at creating a file system watcher for the Go ecosystem. Below are a few that i've encountered in my search:

- [go-fsnotify/fsnotify](https://github.com/go-fsnotify/fsnotify)
- [howeyc/fsnotify](https://github.com/howeyc/fsnotify)
- [jpillora/spy](https://github.com/jpillora/spy)
- [gokyle/fswatch](https://github.com/gokyle/fswatch)
- [rjeczalik/notify](https://github.com/rjeczalik/notify)
- ... and probably a few more

I wanted a library that could live up to the following requirements:

- Work flawlessly on OSX by default: no "too many files open"
- Be able to watch directories with thousands of files in them. As a target, it should work on the Linux source code repository. 
- The simplest possible abstractions, no pipelines, filters or other shenanigans 
- Support recursive watching out of the box, but provide some configuration that can prevent some or all subdirectories from being watched.
- Identical behaviour across OSX, Windows and Linux.

##The Problem
Above requirements seem reasonable but are difficult to meet in practice. As [some](https://github.com/howeyc/fsnotify/issues/54) [discussions](http://lists.qt-project.org/pipermail/development/2012-July/005279.html) have pointed out, the root of the issue lies in the following table of the ideal subsystems for each platform:

platform | subsystem | recursive | event file details 
--- | --- | --- | ---
Linux | inotify | no, not configurable | high
Windows | ReadDirectoryChangesW | configurable | high
OSX | FSEvents | yes, not configurable | low

This 'matrix of hell' makes it difficult to create an abstraction layer on top that is reliable and consistent.

##The Solution
In my opinion one has to simply accept the approach of FSEvent and use its "something happened in a directory" as the abstraction. This effectively delegates the logic for on how to handles events for specific files in that directory to the consumer of the library. 

In practice, this actually makes sense. It often up to the implementation to determine what event constitutes a file change anyway: 

- did the file content actually change or just the timestampe? 
- what do renames actually mean, is it another file or was it moved?
- when files are moved outside the monitored directory, should those be considered as removals?
- what about atomic saves that some IDE's use, are those truly two events or do you want to handle them as a file modification?

Such an abstraction also makes the implementation significantly simpler and allows us to clearly state the guarantees this library offers: 

On any number of changes inside a directory it guarantees _at least_ one event per directory and _at most_ one per latency period. A event is emitted for each single directory, in a scanning scenario you would never need to reexamine it's subdirectories.

## Take it for a spin
Using the library is straight forward:

```Go
import "github.com/timeglass/snow/monitor"

...

//create a monitor for a given root directory with its default configuration
m, err := monitor.New(cwd, nil, 0)

...

//make sure you handle the monitor errors asynchronously
go func() {
	for err := range m.Errors() {
		...
	}
}()

...

//start the monitor, this won't block 
evs, err := m.Start()

...

//handle the directory events anyway you like
for ev := range evs {
	fmt.Println(ev.Dir())
}
```

As another option you could `go get` the super simple main package and run it to see if you like _snow's_ behaviour:

```
go get -u github.com/timeglass/now
```
