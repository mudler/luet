
---
title: "Plugins and Extensions"
linkTitle: "Plugins and Extensions"
weight: 3
description: >
  Extend luet with plugins and extensions
---

Luet can be extended in 2 ways by extensions and plugins.

# Before you begin

You need to have a working `luet` binary installed.

## Extensions

Extensions expand Luet featureset horizontally, so for example, “luet geniso” will allow you to build an iso using luet, without this needing to be part of the luet core.

An Extension is nothing more than a standalone executable file, whose name begins with `luet-`. To install an extension, simply move its executable file to anywhere on your system `PATH`. 

All the plugins will be accessible to luet as `luet pluginname`

### Writing an Extension 

You can write an extension in any programming language or script that allows you to write command-line commands.

Executables receive the inherited environment from luet. An extension determines which command path it wishes to implement based on its name. For example, a plugin wanting to provide a new command luet foo, would simply be named luet-foo, and live somewhere in your PATH.

#### Example Extension

```bash
#!/bin/bash

if [[ "$1" == "help" ]]
then
    echo "Extension help"
    exit 0
fi

if [[ "$1" == "run" ]]
then
    # do something interesting
fi

echo "I am an Extension named luet-foo"

```
### Using an Extension

To use the above extension, simply make it executable:

```bash
$ sudo chmod +x ./luet-foo
```

and place it anywhere in your PATH:

```bash
$ sudo mv ./luet-foo /usr/local/bin
```

You may now invoke your extension as a luet command:

```bash
$ luet foo
I am an Extension named luet-foo
```

All args and flags are passed as-is to the executable:

```bash
$ luet foo help

Extension help
```
## Plugins

Plugins instead are expanding Luet vertically by hooking into internal events. Plugins and Extensions can be written in any language, bash included! Luet uses [go-pluggable](https://github.com/mudler/go-pluggable) so it can dispatch events to external binaries.

Similarly to **Extensions**, a **Plugin** is nothing more than a standalone executable file, but without any special prefix. To install a plugin, simply move its executable file to anywhere on your system `PATH`. 

Differently from **Extensions**, they are not available from the **CLI** and cannot be invoked directly by the user, instead they are called by Luet during its lifecycle.

### Writing a Plugin

You can write a plugin in any programming language or script.

The first argument that is passed to a plugin will always be the event that was emitted by Luet in its lifecycle. You can see all the [events available here](https://github.com/mudler/luet/blob/master/pkg/bus/events.go). The second argument, is a `JSON` encoded payload of the object that Luet is emitting with the event. The object(s) may vary depending on the emitted event.

The output of the plugin (`stdout`) will be parsed as JSON. Every plugin must return a valid JSON at the end of its execution, or it will be marked as failed and stops `luet` further execution. [See also the go-pluggable README](https://github.com/mudler/go-pluggable#plugin-processed-data).

The returning payload should be in the following form:

```json
{ "state": "", "data": "data", "error": ""}
```

By returning a json with the error field not empty, it will make fail the overall execution.


#### Example Plugin

```bash
#!/bin/bash
echo "$1" >> /tmp/event.txt
echo "$2" >> /tmp/payload.txt

echo "{}"

```
### Using a plugin

To use the above plugin, simply make it executable:

```bash
$ sudo chmod +x ./test-foo
```

and place it anywhere in your PATH:

```bash
$ sudo mv ./test-foo /usr/local/bin
```

Now, when running luet, add ```--plugin test-foo```:

```bash

$ luet --plugin test-foo install -y foopackage

```

And check `/tmp/event.txt` to see the event fired and `/tmp/payload.txt` to check the payloads that were emitted by Luet.

### Concrete example

A plugin that prints the images that are being built in `/tmp/exec.log`:

```bash
#!/bin/bash
exec >> /tmp/exec.log
exec 2>&1
event="$1"
payload="$2"
if [ "$event" == "image.post.build" ]; then
  image=$(echo "$payload" | jq -r .data | jq -r .ImageName )
    echo "{ \"data\": \"$image built\" }"
else
    echo "{}"
fi

```
