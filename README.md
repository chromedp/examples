# About chromedp examples

This folder contains a variety of code examples for working with
[`chromedp`][1]. The godoc page contains a number of [simple examples][2] which
are generally self-contained, while this repository holds more complex examples
which tend to require internet access or external components.

Please note that when the `chromedp` package is being rewritten, these examples
may break. Additionally, since these examples are written for specific websites,
there is a good chance that the current selectors, etc break after the website
they are written against changes.

While every effort is made to ensure that these examples are kept up-to-date,
it is expected that the examples made available here may occassionally break.

To file issues, use the [chromedp's issue tracker][3].

## Building and Running an Example

You can build and run these examples in the usual Go way:

```sh
# retrieve examples
$ go get -u -d github.com/chromedp/examples

# run example <prog>
$ go run $GOPATH/src/github.com/chromedp/examples/<prog>/main.go

# build example <prog>
$ go build -o <prog> github.com/chromedp/examples/<prog> && ./<prog>
```
### Available Examples

The following examples are currently available:

<!-- the following section is updated by running `go run gen.go` -->
<!-- START EXAMPLES -->
| Example                   | Description                                                                  |
|---------------------------|------------------------------------------------------------------------------|
| [click](/click)           | use a selector to click on an element                                        |
| [cookie](/cookie)         | set a HTTP cookie on requests                                                |
| [emulate](/emulate)       | emulate a specific device such as an iPhone                                  |
| [eval](/eval)             | evaluate javascript and retrieve the result                                  |
| [headers](/headers)       | set a HTTP header on requests                                                |
| [keys](/keys)             | send key events to an element                                                |
| [logic](/logic)           | more complex logic beyond simple actions                                     |
| [remote](/remote)         | connect to an existing Chrome DevTools instance using a remote WebSocket URL |
| [screenshot](/screenshot) | take a screenshot of a specific element and of the entire browser viewport   |
| [submit](/submit)         | fill out and submit a form                                                   |
| [text](/text)             | extract text from a specific element                                         |
| [upload](/upload)         | upload a file on a form                                                      |
| [visible](/visible)       | wait until an element is visible                                             |
<!-- END EXAMPLES -->

## Contributing

Pull Requests and contributions to this project are encouraged and greatly
welcomed!  The `chromedp` project always needs new examples, and needs talented
developers (such as yourself!) to submit fixes for the existing examples when
they break (for example, when a website's layout/HTML changes).

[1]: https://github.com/chromedp/chromedp
[2]: https://godoc.org/github.com/chromedp/chromedp#pkg-examples
[3]: https://github.com/chromedp/chromedp/issues
