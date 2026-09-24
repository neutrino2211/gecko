# GNote web assets

Edit `notes.template.html`, `notes.css`, and `notes.js`. From the GNote directory,
run `sh scripts/build_web.sh` to regenerate `notes.html`. `gecko build` runs this
step automatically before building the native dependencies.

`notes.html` is a checked-in generated bundle. The native webview loads it as an
HTML string, so its styles and scripts must remain inline. Keeping the bundle
also allows the example to run without a separate asset-loading server.
