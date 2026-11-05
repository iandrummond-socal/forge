# Forge v0.3 sample site

Extract the sample and the binary archive matching your platform. From this sample
folder, run the binary using its full path (use forge.exe on Windows):

```sh
/path/to/forge version
/path/to/forge check
/path/to/forge build
/path/to/forge serve
```

Open http://127.0.0.1:8080 for preview. Output goes to dist/. This sample enables
collections, tags/categories, date archives and pagination in site.toml. Set
indexes.enabled to false and remove page_size to match the older output layout.

Edit the sample content under content/ and layouts under themes/default/. Set
site.base_url to the final HTTPS origin before generating a real site's feeds.
The supplied example.com URL is a placeholder. Publication uses publish_date and
expiry_date; editorial date does not hide future-dated content. Builds are static:
publication/expiry changes on a deployed site require rebuilding and redeploying.

This is the Forge v0.3.0 sample. No account authentication or deployment is
performed by check/build/serve. The deploy command invokes Cloudflare Wrangler
and publishes output; use it only when you intend to publish and have configured
your account and Worker name. Full documentation is in the source archive.
