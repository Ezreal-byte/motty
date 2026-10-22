export default function Page() {
  return (
    <>
      {/* The site is Chinese first: the root sends first visits to the Chinese
          pages, and the switcher in the header is how you leave. The export
          writes zh.html rather than zh/index.html, so the redirect drops the
          trailing slash to match the links the rest of the site emits. */}
      <meta httpEquiv="refresh" content="0;url=/zh" />
      <p>
        <a href="/zh">简体中文</a> · <a href="/en">English</a>
      </p>
    </>
  );
}
