# Go HTTP Logger Middleware

This is a Go logger middleware geared towards capturing the important per-request
data while allowing clients to specify _how_ their logging takes place.

This is mostly helpful when you have a specific format for other logs in your
app that you want to continue to follow, while you don't want to implement the
required logic for actually grabbing the data yourself.

Loosely based on https://blog.questionable.services/article/guide-logging-middleware-go/

See the code for the [StdLibLogger](/stdlib_logger.go) for an example of how to
create your own logger.
