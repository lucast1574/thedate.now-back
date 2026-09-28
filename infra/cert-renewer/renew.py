"""Renew the wedding wildcard and publish it to Dokploy's Traefik config."""

import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.parse
import urllib.request
from datetime import datetime, timezone


ROOT = Path("/acme")
DOMAIN = "*.save.thedate.now"
CERT = ROOT / "certificates" / "_.save.thedate.now.crt"
ISSUER = ROOT / "certificates" / "_.save.thedate.now.issuer.crt"
KEY = ROOT / "certificates" / "_.save.thedate.now.key"
UPLOADED = ROOT / "uploaded.sha256"


def api(path, body=None):
    base = os.environ["DOKPLOY_URL"].rstrip("/")
    headers = {"x-api-key": os.environ["DOKPLOY_API_KEY"]}
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode()
    request = urllib.request.Request(
        f"{base}/api/{path}",
        headers=headers,
        data=data,
        method="POST" if body is not None else "GET",
    )
    with urllib.request.urlopen(request, timeout=60) as response:
        content = response.read()
    return json.loads(content) if content else None


def renew():
    args = [
        "lego",
        "--path", str(ROOT),
        "--email", os.environ["ACME_EMAIL"],
        "--dns", "cloudflare",
        "--domains", DOMAIN,
        "--dns.resolvers", "1.1.1.1:53,8.8.8.8:53",
        "--accept-tos",
    ]
    args += ["renew", "--days", "30"] if CERT.exists() else ["run"]
    subprocess.run(args, check=True, timeout=900)


def publish():
    fullchain = CERT.read_bytes() + ISSUER.read_bytes()
    private_key = KEY.read_bytes()
    digest = hashlib.sha256(fullchain + private_key).hexdigest()
    if UPLOADED.exists() and UPLOADED.read_text().strip() == digest:
        print("Certificate unchanged; no upload needed", flush=True)
        return

    api("certificates.update", {
        "certificateId": os.environ["CERTIFICATE_ID"],
        "certificateData": fullchain.decode(),
        "privateKey": private_key.decode(),
    })

    app_id = os.environ["WEDDING_APP_ID"]
    query = urllib.parse.urlencode({"applicationId": app_id})
    config = api(f"application.readTraefikConfig?{query}")
    if not isinstance(config, str) or "tls:" not in config:
        raise RuntimeError("Wedding Traefik config has no certificate registration")
    # Touch the dynamic config after updating the PEM files so Traefik reloads them.
    marker = f"# wildcard certificate refreshed {datetime.now(timezone.utc).isoformat()}"
    lines = [line for line in config.splitlines() if not line.startswith("# wildcard certificate refreshed ")]
    api("application.updateTraefikConfig", {
        "applicationId": app_id,
        "traefikConfig": "\n".join(lines) + "\n" + marker + "\n",
    })
    UPLOADED.write_text(digest + "\n")
    print("Published renewed wildcard certificate", flush=True)


if __name__ == "__main__":
    try:
        renew()
        publish()
    except Exception as error:
        print(f"Certificate renewal failed: {error}", file=sys.stderr, flush=True)
        raise
