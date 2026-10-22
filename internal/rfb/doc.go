// Package rfb is a client for the Remote Framebuffer protocol (RFC 6143), the
// protocol of VNC. It has no dependency on a user interface: pixels go to a
// Sink, input comes in through the Send methods, and package
// internal/desktop/vnc connects the two to the workbench.
//
// What it speaks: protocol versions 3.3, 3.7 and 3.8; the security types None
// and VNC Authentication; a 32-bit true-colour pixel format that it asks the
// server for, so no pixel is ever converted; the encodings Raw, CopyRect, RRE,
// Hextile and ZRLE; the DesktopSize pseudo-encoding; pointer, key and
// clipboard (Latin-1 text) messages. What it does not: VeNCrypt and other TLS
// security types, Apple Remote Desktop authentication, the cursor shape
// pseudo-encoding (the server then draws the cursor into the framebuffer),
// extended clipboard, Tight and the other encodings, audio.
//
// A server is not trusted. Every length it sends is checked against a limit
// before memory is allocated for it, every rectangle against the framebuffer,
// and no decoder reads further than the structure it is decoding says.
package rfb
