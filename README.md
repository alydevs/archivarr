archivarr
=========

* This tool is ***only*** to be used in a local/homelab [Servarr](https://servarr.com) stack to facilitate mirroring of archive.org collections that require a login to access.
* **DO NOT** host an instance of this tool publicly.
* The developer(s) of this tool are not responsible for any copyright infringement undertaken using this tool.

Usage
-----
* Once your instance of archivarr is running, you can make requests to download things using it, replacing `https://archive.org/download/` with `http://127.0.0.1:8080/`.
* You can also use this as a torrent web seed source to download login-restricted collections via torrents.

Compose
-------

```yaml
services:
  archivarr:
    image: ghcr.io/alydevs/archivarr:latest
    environment:
      - IA_ACCESS=your_access_key
      - IA_SECRET=your_secret_key
    ports:
      - "8080:8080"
```

Disclosure
----------
Claude Sonnet 4.6 was used in the creation of archivarr.go and Dockerfile.
