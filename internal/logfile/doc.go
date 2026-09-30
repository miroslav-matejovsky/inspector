// Package logfile writes the runtime log of one run to its own file.
//
// Open creates the file <name>-<yyyyMMdd-HHmmss>.log in a directory, creating
// the directory when it is missing. It never overwrites a file: a second run
// started in the same second fails.
//
// The logger uses the log/slog text handler: one record per line with time,
// level, msg and then the attributes as key=value pairs. Each record is one
// write to the unbuffered file.
//
// The caller owns the file and closes it after the last record.
package logfile
