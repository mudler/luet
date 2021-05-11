# :bento: go-pluggable
[![PkgGoDev](https://pkg.go.dev/badge/github.com/mudler/go-pluggable)](https://pkg.go.dev/github.com/mudler/go-pluggable) [![Go Report Card](https://goreportcard.com/badge/github.com/mudler/go-pluggable)](https://goreportcard.com/report/github.com/mudler/go-pluggable) [![Test](https://github.com/mudler/go-pluggable/workflows/Test/badge.svg)](https://github.com/mudler/go-pluggable/actions?query=workflow%3ATest)

:bento: *go-pluggable* is a light Bus-event driven plugin library for Golang.

`go-pluggable` implements the event/sub pattern to extend your Golang project with external binary plugins that can be written in any language.

```golang
import "github.com/mudler/go-pluggable"


func main() {

    var myEv pluggableEventType = "something.to.hook.on"
    temp := "/usr/custom/bin"

    m = pluggable.NewManager(
        []pluggable.EventType{
            myEv,
        },
    )
        
    // We have a file 'test-foo' in temp.
    // 'test-foo' will receive our event payload in json
    m.Autoload("test", temp)
    m.Register()

    // Optionally process plugin results response
    // The plugins has to return as output a json in stdout in the format { 'state': "somestate", data: "some data", error: "some error" }
    // e.g. with jq:  
    // jq --arg key0   'state' \
    // --arg value0 '' \
    // --arg key1   'data' \
    // --arg value1 "" \
    // --arg key2   'error' \
    // --arg value2 '' \
    // '. | .[$key0]=$value0 | .[$key1]=$value1 | .[$key2]=$value2' \
    // <<<'{}'
    m.Response(myEv, func(p *pluggable.Plugin, r *pluggable.EventResponse) { ... }) 

    // Emit events, they are encoded and passed as JSON payloads to the plugins.
    // In our case, test-foo will receive the map as JSON
    m.Publish(myEv,  map[string]string{"foo": "bar"})


}

```
