# streamer

Simple WebSocket server and client with standard stream

## Usage

```
# Server
$ streamer
foo

# Client
$ echo 'foo' | streamer -c
```

### Set port number

```
# Server
$ streamer -p 5500
foo

# Client
$ echo 'foo' | streamer -c -p 5500
```

### Use binary data transfer

```
# Server
$ streamer -b
foo

# Client
$ echo -n -e '\x66\x6f\x6f' | streamer -b -c
```

## Installation

```
$ go get github.com/ionstage/streamer
```

## License
&copy; 2023 iOnStage
Licensed under the MIT License.
