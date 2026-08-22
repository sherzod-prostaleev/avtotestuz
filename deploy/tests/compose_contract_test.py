#!/usr/bin/env python3
"""Static, secret-safe assertions for deployment Compose contracts."""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]


def compose_config(files: list[str], env_file: str, extra_env: dict[str, str] | None = None) -> dict:
    command = ["docker", "compose"]
    for file in files:
        command.extend(["-f", file])
    command.extend(
        ["--env-file", env_file, "config", "--no-env-resolution", "--format", "json"]
    )
    env = os.environ.copy()
    env.update(extra_env or {})
    # Compose services pin env_file: app.env (required). CI has no host secrets
    # file; materialize a throwaway copy from the example used for --env-file.
    app_env = ROOT / "deploy" / "app.env"
    created_app_env = False
    if not app_env.exists():
        source = Path(env_file)
        if not source.is_absolute():
            source = ROOT / source
        shutil.copyfile(source, app_env)
        os.chmod(app_env, 0o600)
        created_app_env = True
    try:
        output = subprocess.check_output(command, cwd=ROOT, env=env, text=True)
    finally:
        if created_app_env and app_env.exists():
            app_env.unlink()
    return json.loads(output)


class ComposeContractTest(unittest.TestCase):
    def test_local_infra_is_loopback_only(self) -> None:
        config = compose_config(["docker-compose.yml"], "deploy/app.env.example")
        for service_name in ("postgres", "redis", "minio"):
            for port in config["services"][service_name].get("ports", []):
                self.assertEqual(port["host_ip"], "127.0.0.1", (service_name, port))

    def test_prod_web_receives_only_frontend_environment(self) -> None:
        config = compose_config(
            ["deploy/docker-compose.prod.yml"], "deploy/app.prod.env.example"
        )
        web = config["services"]["web"]
        self.assertNotIn("env_file", web)
        self.assertEqual(
            set(web["environment"]),
            {
                "BACKEND_URL",
                "CLIENT_IP_ASSERTION_SECRET",
                "KEEP_ALIVE_TIMEOUT",
                "NEXT_PUBLIC_SENTRY_DSN",
                "TRUSTED_PROXY_HOPS",
                "WEB_WORKERS",
            },
        )
        api = config["services"]["api"]
        for key in (
            "MINIO_ENDPOINT",
            "MINIO_ACCESS_KEY",
            "MINIO_SECRET_KEY",
            "MINIO_SUPPORT_BUCKET",
            "MINIO_LEGACY_SUPPORT_BUCKET",
        ):
            self.assertIn(key, api["environment"])
        self.assertTrue(api["read_only"])
        self.assertEqual(api["depends_on"]["minio-init"]["condition"], "service_completed_successfully")
        self.assertNotIn("default", web["networks"])
        self.assertNotIn("app", config["services"]["postgres"]["networks"])
        self.assertEqual(config["networks"]["default"]["name"], "drivergo_default")

    def test_minio_policy_and_overlay_api_health_contract(self) -> None:
        config = compose_config(
            ["docker-compose.yml", "deploy/docker-compose.app.yml"],
            "deploy/app.env.example",
        )
        init_command = " ".join(config["services"]["minio-init"]["entrypoint"])
        self.assertIn("support-attachments", init_command)
        self.assertIn("anonymous set-json /tmp/media-public-policy.json local/media", init_command)
        self.assertIn('"arn:aws:s3:::media/images/*"', init_command)
        self.assertIn("mc version enable", init_command)
        self.assertNotIn("anonymous set download local/media;", init_command)
        api = config["services"]["api"]
        self.assertIn("healthcheck", api)
        self.assertEqual(api["depends_on"]["minio-init"]["condition"], "service_completed_successfully")

    def test_candidate_is_app_only_and_requires_fixed_images(self) -> None:
        config = compose_config(
            ["deploy/docker-compose.candidate.yml"],
            "deploy/app.prod.env.example",
            {
                "CANDIDATE_API_IMAGE": "sha256:" + "0" * 64,
                "CANDIDATE_WEB_IMAGE": "sha256:" + "1" * 64,
            },
        )
        self.assertEqual(set(config["services"]), {"api-candidate", "web-candidate"})
        self.assertEqual(config["services"]["api-candidate"]["pull_policy"], "never")
        self.assertEqual(config["services"]["web-candidate"]["pull_policy"], "never")
        self.assertEqual(config["services"]["api-candidate"]["restart"], "unless-stopped")
        self.assertEqual(config["services"]["web-candidate"]["restart"], "unless-stopped")

    def test_support_migration_is_copy_only(self) -> None:
        script = (ROOT / "deploy/migrate-support-bucket.sh").read_text()
        self.assertIn("mc mirror --dry-run --overwrite", script)
        self.assertIn('mc cp "$source" "$target"', script)
        self.assertIn('mc stat --json "$target"', script)
        self.assertIn('"Code":"NoSuchKey"', script)
        self.assertIn("target bucket versioning is not enabled", script)
        self.assertIn('source_digest="$(object_sha256 "$source")"', script)
        self.assertIn('target_digest="$(object_sha256 "$target")"', script)
        self.assertIn("collision: existing target differs", script)
        self.assertNotIn("\n  mc mirror --overwrite ", script)
        self.assertNotIn("mc mirror --remove", script)
        self.assertNotIn("mc rm ", script)
        self.assertIn("MINIO_BUCKET:-support-attachments", script)

    def test_legacy_private_bucket_contract_is_preserved(self) -> None:
        config = compose_config(
            ["deploy/docker-compose.prod.yml"],
            "deploy/app.prod.env.example",
            {"MINIO_SUPPORT_BUCKET": "", "MINIO_BUCKET": "existing-private"},
        )
        for service_name in ("api", "minio-init"):
            environment = config["services"][service_name]["environment"]
            self.assertEqual(environment["MINIO_SUPPORT_BUCKET"], "")
            self.assertEqual(environment["MINIO_BUCKET"], "existing-private")

    def test_candidate_operator_pins_single_api_and_does_not_run_migrations(self) -> None:
        script = (ROOT / "deploy/candidate-app.sh").read_text()
        self.assertIn("--scale api-candidate=1", script)
        self.assertNotIn(" migrate ", script)
        self.assertIn("lock_and_require_stable_slot", script)
        self.assertNotIn(':[0-9a-f]{12,40}', script)

    def test_staging_web_waits_for_api_readiness(self) -> None:
        config = compose_config(
            ["docker-compose.yml", "deploy/docker-compose.app.yml"],
            "deploy/app.env.example",
        )
        self.assertEqual(config["services"]["web"]["depends_on"]["api"]["condition"], "service_healthy")

    def test_web_healthcheck_is_cheap_probe(self) -> None:
        for rel in (
            "deploy/docker-compose.prod.yml",
            "deploy/docker-compose.app.yml",
            "deploy/docker-compose.candidate.yml",
        ):
            text = (ROOT / rel).read_text(encoding="utf-8")
            self.assertIn("/api/healthz", text, rel)
            self.assertNotIn("127.0.0.1:3000/uz-Latn", text, rel)
            self.assertNotIn("127.0.0.1:3000/uz-Cyrl", text, rel)

    def test_nginx_html_keepalive_and_closed_media_listing(self) -> None:
        conf = (ROOT / "deploy/nginx-drivergo.uz.conf").read_text(encoding="utf-8")
        self.assertIn("map $http_upgrade $drivergo_connection_upgrade", conf)
        self.assertIn("proxy_set_header   Connection $drivergo_connection_upgrade", conf)
        self.assertIn("location = /media/", conf)
        self.assertIn("location /_next/static/", conf)
        listing = conf.split("location = /media/", 1)[1].split("location ", 1)[0]
        self.assertIn("return 404", listing)

    def test_next_outlives_nginx_upstream_keepalive(self) -> None:
        """Node must close a pooled connection LATER than nginx does.

        nginx ignores the upstream's `Keep-Alive: timeout=` header entirely and
        recycles a pooled socket for up to its own `keepalive_timeout`. Node's
        default is 5s. Enabling `keepalive 16` on drivergo_web (c1b5754,
        2026-08-15) therefore handed nginx a 55s window in which every pooled
        socket had already been closed by Next: nginx wrote the request into a
        dead socket and logged "upstream prematurely closed connection". GET was
        retried and healed silently, but POST is non-idempotent, so nginx never
        retried it and the browser got a bare 502 -- which is why the failures
        landed on POST /api/proxy/sessions/{id}/answers (answering a question)
        and not on page loads.

        The invariant is one-directional: whoever nginx talks to must hold the
        socket open longer than nginx will reuse it, so nginx is always the side
        that closes. Equal values are not enough -- the loser of that tie is
        decided by clock skew and scheduling latency.
        """
        upstreams = (ROOT / "deploy/nginx/upstreams-stable.conf").read_text(encoding="utf-8")
        web_block = upstreams.split("upstream drivergo_web", 1)[1].split("}", 1)[0]
        nginx_match = re.search(r"keepalive_timeout\s+(\d+)s", web_block)
        self.assertIsNotNone(
            nginx_match,
            "drivergo_web must pin keepalive_timeout explicitly; the 60s default "
            "is invisible at the call site and cannot be reasoned about here",
        )
        nginx_ms = int(nginx_match.group(1)) * 1000

        compose = (ROOT / "deploy/docker-compose.prod.yml").read_text(encoding="utf-8")
        node_match = re.search(r"KEEP_ALIVE_TIMEOUT:\s*\$\{[A-Z_]+:-(\d+)\}", compose)
        self.assertIsNotNone(
            node_match,
            "web must set KEEP_ALIVE_TIMEOUT (ms); without it Next.js keeps "
            "Node's 5s default and races nginx's pooled sockets",
        )
        node_ms = int(node_match.group(1))

        self.assertGreater(
            node_ms,
            nginx_ms,
            f"Next.js closes pooled sockets at {node_ms}ms but nginx reuses them "
            f"for up to {nginx_ms}ms -- POSTs will 502 in that window",
        )

    def test_access_log_records_upstream_timings(self) -> None:
        """A 502 must be attributable to an upstream, and slow must be visible.

        Under the inherited `combined` format the only evidence of the
        2026-08-14 keep-alive regression was error.log; access.log could not say
        which upstream lost the request, nor whether it was slow or instant.
        """
        conf = (ROOT / "deploy/nginx-drivergo.uz.conf").read_text(encoding="utf-8")
        self.assertIn("log_format drivergo_timed", conf)
        for field in (
            "$request_time",
            "$upstream_response_time",
            "$upstream_status",
            "$upstream_addr",
        ):
            self.assertIn(field, conf, field)
        self.assertIn(
            "access_log /var/log/nginx/drivergo-access.log drivergo_timed", conf
        )

    def test_station_agent_download_is_streamed_and_rate_limited(self) -> None:
        """The self-update download must keep the station rate limit.

        It is ~6 MB and it sits on a prefix that is otherwise all small JSON
        control calls, so it gets its own exact-match block. The risk in
        splitting a location out is dropping something the prefix block was
        providing: without limit_req an attacker would have found a cheaper
        route to 6 MB than to a token request, and with buffering left on
        nginx would spool every classroom's download through a temp file.
        """
        conf = (ROOT / "deploy/nginx-drivergo.uz.conf").read_text(encoding="utf-8")
        self.assertIn("location = /api/v1/b2b/stations/agent {", conf)
        block = conf.split("location = /api/v1/b2b/stations/agent {", 1)[1].split("\n    }", 1)[0]
        self.assertIn("limit_req          zone=station_ip", block)
        self.assertIn("proxy_buffering    off", block)
        self.assertIn("proxy_pass         http://drivergo_api", block)

    def test_station_version_is_not_a_forgettable_build_arg(self) -> None:
        """backend/station/VERSION is the version of record, not the shell.

        A non-empty default here silently wins over the file, which is exactly
        how every agent built after 2026-08-07 shipped reporting 1.0.0 while
        three different builds were in the field.
        """
        for rel in ("deploy/docker-compose.prod.yml", "deploy/docker-compose.app.yml"):
            text = (ROOT / rel).read_text(encoding="utf-8")
            self.assertIn("STATION_VERSION: ${STATION_VERSION:-}", text, rel)
        version = (ROOT / "backend/station/VERSION").read_text(encoding="utf-8").strip()
        self.assertRegex(version, r"^\d+\.\d+\.\d+$")


if __name__ == "__main__":
    unittest.main()
