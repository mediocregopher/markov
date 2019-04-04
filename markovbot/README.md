# markovbot

A simple slack bot which writes all messages from channels into chains on a
running markov api instance. Each channel's chain is independant of every other
channel's.

## Build

See build instructions in the top-level package for version info:

    go build .

## Setup

markovbot requires a running instance of [markov](https://github.com/mediocregopher/markov) in order to work. Once done
you'll need to make a slack "bot" and grab its token

    # Describes options which can be passed in. -token is required
    ./markovbot -h

    # To actually run it
    ./markovbot -token mytoken

Once running you should be able to invite your bot into channels and it will
automatically start listening to conversation.

## Usage

markovbot will automatically separate the text it generates by channel, so text
from one channel will not be used when generating text for another.

Any message beginning with `markov` will cause markovbot to interject with a
random sentence. markovbot will also interject randomly in the conversation, see
the `-interject-wait` argument.
