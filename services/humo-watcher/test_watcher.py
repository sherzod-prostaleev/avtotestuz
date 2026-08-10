from __future__ import annotations

import asyncio
import errno
from pathlib import Path
import tempfile
import unittest
from unittest.mock import AsyncMock, patch

import watcher


class DurableSpoolTest(unittest.TestCase):
    def test_queue_is_durable_ordered_and_idempotent(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "queue.sqlite3"
            spool = watcher.DurableSpool(str(path))

            spool.put(20, "second")
            spool.put(10, "first")
            spool.put(10, "duplicate-must-not-replace")

            self.assertEqual(spool.first(), (10, "first"))
            spool.ack(10)
            self.assertEqual(spool.first(), (20, "second"))
            spool.ack(20)
            self.assertIsNone(spool.first())

            reopened = watcher.DurableSpool(str(path))
            self.assertIsNone(reopened.first())


class HealthServerTest(unittest.IsolatedAsyncioTestCase):
    async def test_event_loop_health_endpoint(self) -> None:
        class MemoryWriter:
            def __init__(self) -> None:
                self.buffer = bytearray()
                self.closed = False

            def write(self, data: bytes) -> None:
                self.buffer.extend(data)

            async def drain(self) -> None:
                return None

            def close(self) -> None:
                self.closed = True

            async def wait_closed(self) -> None:
                return None

        reader = asyncio.StreamReader()
        reader.feed_data(b"GET /healthz HTTP/1.1\r\nHost: localhost\r\n\r\n")
        reader.feed_eof()
        writer = MemoryWriter()
        await watcher.serve_health(reader, writer)  # type: ignore[arg-type]
        response = bytes(writer.buffer)

        self.assertIn(b"HTTP/1.1 200 OK", response)
        self.assertIn(b'{"status":"ok"}', response)
        self.assertTrue(writer.closed)


class CriticalTaskSupervisorTest(unittest.IsolatedAsyncioTestCase):
    async def test_worker_failure_stops_sibling_and_propagates(self) -> None:
        sibling_cleaned = asyncio.Event()

        async def fail_worker() -> None:
            await asyncio.sleep(0)
            raise OSError("fixture spool failure")

        async def blocked_sibling() -> None:
            try:
                await asyncio.Event().wait()
            finally:
                sibling_cleaned.set()

        failed = asyncio.create_task(fail_worker(), name="retry-spool")
        sibling = asyncio.create_task(blocked_sibling(), name="telegram-client")
        with self.assertRaisesRegex(RuntimeError, "critical task retry-spool failed") as raised:
            await watcher.supervise_critical_tasks(failed, sibling)

        self.assertIsInstance(raised.exception.__cause__, OSError)
        self.assertTrue(sibling.cancelled())
        self.assertTrue(sibling_cleaned.is_set())

    async def test_unexpected_clean_worker_exit_is_fatal(self) -> None:
        async def exits() -> None:
            return None

        task = asyncio.create_task(exits(), name="retry-spool")
        with self.assertRaisesRegex(RuntimeError, "exited unexpectedly"):
            await watcher.supervise_critical_tasks(task)


class CallbackFailureSupervisionTest(unittest.IsolatedAsyncioTestCase):
    async def test_disk_full_spool_callback_failure_terminates_worker(self) -> None:
        class DiskFullSpool:
            def put(self, msg_id: int, raw_text: str) -> None:
                raise OSError(errno.ENOSPC, "No space left on device")

        class Event:
            id = 42
            raw_text = "To'ldirish 100000 so'm"

        class FakeTelegramClient:
            handler = None

            def __init__(self, *_args: object) -> None:
                pass

            def on(self, _event: object):
                def register(handler):
                    self.handler = handler
                    return handler

                return register

            async def start(self) -> None:
                return None

            async def get_me(self) -> object:
                return type("User", (), {"id": 1, "username": "fixture"})()

            async def run_until_disconnected(self) -> None:
                assert self.handler is not None
                asyncio.create_task(self.handler(Event()))
                await asyncio.sleep(0)
                await asyncio.Event().wait()

            async def disconnect(self) -> None:
                return None

        with (
            patch.object(watcher, "TelegramClient", FakeTelegramClient),
            patch.object(watcher, "StringSession", lambda value: value),
            patch.object(
                watcher,
                "load_credentials",
                new=AsyncMock(
                    return_value={"api_id": 1, "api_hash": "hash", "session": "session"}
                ),
            ),
        ):
            telegram_worker = asyncio.create_task(
                watcher.watch_credentials_loop(
                    object(),  # type: ignore[arg-type]
                    DiskFullSpool(),  # type: ignore[arg-type]
                    asyncio.Lock(),
                ),
                name="telegram-client",
            )
            retry_worker = asyncio.create_task(
                asyncio.Event().wait(), name="retry-spool"
            )
            with self.assertRaisesRegex(
                RuntimeError, "critical task telegram-client failed"
            ) as raised:
                await asyncio.wait_for(
                    watcher.supervise_critical_tasks(retry_worker, telegram_worker),
                    timeout=1,
                )

        self.assertIsInstance(raised.exception.__cause__, watcher.SpoolWriteError)
        self.assertIsInstance(raised.exception.__cause__.__cause__, OSError)


if __name__ == "__main__":
    unittest.main()
