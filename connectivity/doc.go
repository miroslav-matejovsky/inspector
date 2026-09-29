// Package connectivity is the Connectivity domain of Inspector: it gives
// read-only access to the sources of observations.
//
// Core questions:
//
//	Where does the information come from?
//	How is it accessed?
//	How is it connected?
//
// # Read-only
//
// HTTPReader has no method that sends anything but a GET request, and every
// path stays under its base URL. This enforces the Observe principle at the
// boundary: Inspector never modifies what it inspects.
//
// # Documents
//
// Get returns every HTTP status as a Document; the caller decides which
// statuses are acceptable, because an error status with a JSON body can be a
// meaningful observation. The response must have the media type
// application/json and a body of at most MaxDocumentBytes. Each Get is
// bounded by the timeout of the reader. Document.Decode ignores unknown
// fields, so a source can add fields without breaking its readers.
//
// # Errors
//
// All errors are system failures of the source access. They carry the URL
// and wrap the original error, for example context.DeadlineExceeded.
package connectivity
