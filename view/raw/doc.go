// Package raw is the raw view: it presents the signals of a source.Source as
// they were collected, before any model is built from them.
//
// Render is composed of smaller views:
//
//   - targets: a table with one row per target: stored signals, then time,
//     status, duration, body size, content type and error of the latest
//     signal. The URL of a target is the title of its name, and the name of
//     a target with a signal links to its signal view.
//   - signal: the latest signal of one target, with the id
//     "signal-<target>": name, status, URL, time, duration, size and content
//     type, then the error of a failed read or the body.
//   - body: a response body formatted by its content type. JSON is indented
//     and highlighted; Prometheus and OpenMetrics text is highlighted per
//     sample (metric name, labels, value) and comment; anything else is plain
//     text. A body that is not valid JSON is shown as text with the parse
//     error. Only the first 500 lines of a formatted body are shown, followed
//     by the number of lines left out. Invalid UTF-8 is replaced by U+FFFD.
//
// The targets table spans the full width; the signal views follow below it in
// two columns, one column on screens up to 1100px wide.
//
// Every body that scrolls carries a data-scroll attribute with a key that is
// the same in every render: "<signal id>-body". A page that replaces a
// rendered view with a newer one can use the keys to keep the scroll
// positions.
//
// A signal is marked ok when a response with a status below 400 was read,
// bad when the read failed or the status is 400 or above, and none when the
// target has no stored signal.
//
// A page includes view.Styles and Styles.
package raw
