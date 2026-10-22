# Sidebar lockup on a direct home-page launch

Verified on 2026-10-22. Companion Viva change: branch
`codex/sidebar-logo-startup`, documented in Viva's `docs/initial-frame.md`.

## Ownership and cause

Motty owns the embedded lockup, theme choice, and sidebar layout. Viva owns
native image-resource residency and upload decisions. Motty pre-paints its
canvas before opening an application-managed window. Viva reconciles that
canvas, then paints again with the native text engine. When this second
preparation pass changes text/layout but not the logo, the old painter marks
the logo as retained even though the native renderer has received no pixels.
Leaving the page and returning supplies the missing upload later.

Viva now uploads every bitmap in the final initial frame. Motty needs no
production-code change, navigation shim, redraw request, timer, or ID churn.
The public driver's `Label.BitmapUpload` diagnostic lets the application test
check the actual native upload decision, rather than just the presence of
image bytes in a canvas.

## Exercised behavior

- Viva's first-frame regression fails without the upload fix when the second
  preparation pass changes a caption but retains the bitmap.
- Motty's real startup paint followed by `headless.Open` fails without the fix
  for both light and dark themes; with the fix the initial frame uploads the
  visible sidebar lockup without navigation. Advancing idle time causes no paint.
- The macOS `vivahead` test sends the actual prepared window frame to the native
  Metal staging path, not the headless snapshot converter. Without the fix its
  bitmap pixel is `[0 0 0 255]`; with the fix it is `[0 0 255 255]` (BGRA).
- A fresh temporary-home headless application launch and a light-theme relaunch
  both show the logo on their first screenshot, taken with `shot PATH now`.
  Both `layout` checks report zero problems.
- The focused Viva and Motty startup tests also pass on the shared Debian 12
  x86_64 and Windows 11 x86_64 machines without opening application windows.
- Viva's full format/vet/test gates, headless vet, Windows vet, and WebAssembly
  vet pass. Motty's full format/vet/test/race gates pass.

Artifacts are preserved in the task worktree's `artifacts/` directory:
first-frame screenshots, headless scripts/logs, before/after Metal results,
platform logs, gate logs, and stripped binaries.

## Costs and limits

On an Apple M2 Pro, macOS arm64, `go test -bench BenchmarkBitmapStartup
-benchtime=1s -count=3` measures an unchanged repaint at a median 107.9 ns before
and 105.9 ns after, with 0 B/op and 0 allocs/op in both. Initial preparation
uses 7,304 B/op and 27 allocs/op in both. Initial timing ranges are 1.650–1.667 µs
before and 1.606–1.910 µs after; the variation does not support a startup-speed
claim. The runtime painter gains no persistent field or cache. The diagnostic
`app.Label` remains 184 bytes on arm64 because its new flag uses existing padding.
Viva's idle-driver test verifies no added goroutines and no work after settling.

Native stripped Motty builds (`go build -trimpath -ldflags='-s -w' ./cmd/motty`)
are 21,416,818 bytes before and after. Motty's warmed paint allocation test also
passes. No new dependency, poller, timer, background worker, or frame request is
introduced.

No real application window was opened. Real-window compositing/presentation on
macOS, Windows and Linux is not claimed; macOS image staging and pixels were
verified offscreen. SSH, keyboard and IME behavior are unrelated to this change
and were not manually exercised.
