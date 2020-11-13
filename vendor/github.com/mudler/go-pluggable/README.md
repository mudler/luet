# go-pluggable

light Bus-event driven plugin library for Golang.

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

    // ...
    m.Publish(myEv,  map[string]string{"foo": "bar"}) // test-foo, will receive our data as json payload
}

```