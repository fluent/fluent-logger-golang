# CHANGELOG

## 1.6.1
* Add another fix for `Close` called twice in Async

## 1.6.0
* Add support for `ppc64le`
* Fix unexpected behaviors&panic around `Close`

## 1.5.0
* Add `ForceStopAsyncSend` to stop asynchronous message transferring immediately when `close()` called
* Fix to lock connections only when needed
* Fix a bug to panic for closing nil connection

## 1.4.0
* Add `RequestAck` to enable at-least-once message transferring
* Add `Async` option to update sending message in asynchronous way
* Deprecate `AsyncConnect` (Use `Async` instead)

## 1.3.0
* Add `SubSecondPrecision` option to handle Fluentd v0.14 (and v1) sub-second EventTime (default: false)
* Add `WriteTimeout` option
* Fix API of `Post` to accept `msgp.Marshaler` objects for better performance

## 1.2.1
* Fix a bug not to reconnect to destination twice or more
* Fix to connect on background goroutine in async mode

## 1.2.0
* Add `MarshalAsJSON` feature for `message` objects which can be marshaled as JSON
* Fix a bug to panic for destination system outage

## 1.1.0
 * Add support for unix domain socket
 * Add asynchronous client creation

## 1.0.0
 * Fix API of `Post` and `PostWithTime` to return error when encoding
 * Add argument checks to get `map` with string keys and `struct` only
 * Logger refers tags (`msg` or `codec`) of fields of struct

## 0.6.0
 * Change dependency from ugorji/go/codec to tinylib/msgp
 * Add `PostRawData` method to post pre-encoded data to servers

## 0.5.1
 * Lock when writing pending buffers (Thanks @eagletmt)

## 0.5.0
 * Add TagPrefix in Config (Thanks @hotchpotch)

## 0.4.4
 * Fixes runtime error of close function.(Thanks @y-matsuwitter)

## 0.4.3
 * Added method PostWithTime(Thanks [@choplin])

## 0.4.2
 * Use sync.Mutex
 * Fix BufferLimit comparison
 * Export toMsgpack function to utils.go

## 0.4.1
 * Remove unused fmt.Println

## 0.4.0
 * Update msgpack library ("github.com/ugorji/go-msgpack" -> "github.com/ugorji/go/codec")
