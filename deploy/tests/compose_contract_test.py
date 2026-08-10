#!/usr/bin/env python3
"""Static, secret-safe assertions for deployment Compose contracts."""

from __future__ import annotations

import json
import os
from pathlib import Path
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
                "NEXT_PUBLIC_SENTRY_DSN",
                "TRUSTED_PROXY_HOPS",
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


if __name__ == "__main__":
    unittest.main()
