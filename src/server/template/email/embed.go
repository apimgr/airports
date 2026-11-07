// Package email embeds the built-in operator notification templates.
package email

import "embed"

// Templates contains default email templates shipped with the server binary.
//
//go:embed *.txt
var Templates embed.FS
