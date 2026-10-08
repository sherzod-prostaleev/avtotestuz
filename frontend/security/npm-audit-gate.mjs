// High/critical npm advisory gate with reviewed, time-boxed exceptions.
//
// `npm audit --audit-level=high` has no way to record an accepted risk, so a
// single advisory with no published fix anywhere in the ecosystem turns CI red
// with nothing left to upgrade. This gate reads the `npm audit --json` report
// and fails on:
//   * any high/critical advisory that is not listed in the exceptions file;
//   * an exception whose `expires` date has passed;
//   * an exception granted for longer than MAX_EXCEPTION_DAYS;
//   * a stale exception (its advisory no longer appears — remove it);
//   * a report that is not a well-formed audit report (network/registry errors
//     must never read as "no vulnerabilities").
//
// Production dependencies are gated separately in CI with a plain
// `npm audit --omit=dev --audit-level=high`, which takes no exceptions: an
// exception here can only ever cover build/lint tooling.
//
// Usage: node security/npm-audit-gate.mjs <npm-audit.json> <exceptions.json>

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export const BLOCKING_SEVERITIES = new Set(["high", "critical"]);
export const MAX_EXCEPTION_DAYS = 90;

const GHSA = /^GHSA(-[23456789cfghjmpqrvwx]{4}){3}$/;
const DATE = /^\d{4}-\d{2}-\d{2}$/;
const DAY_MS = 24 * 60 * 60 * 1000;

function advisoryId(url) {
  const id = String(url ?? "").split("/").pop();
  return GHSA.test(id) ? id : null;
}

function parseDate(value, field, problems, who) {
  if (typeof value !== "string" || !DATE.test(value)) {
    problems.push(`${who}: ${field} must be a YYYY-MM-DD date`);
    return null;
  }
  const ms = Date.parse(`${value}T00:00:00Z`);
  if (Number.isNaN(ms)) {
    problems.push(`${who}: ${field} is not a real date`);
    return null;
  }
  return ms;
}

/**
 * Collect the distinct advisories (not the dependents that merely inherit
 * them) at a blocking severity.
 */
export function blockingAdvisories(report) {
  if (
    report === null ||
    typeof report !== "object" ||
    report.auditReportVersion !== 2 ||
    report.vulnerabilities === null ||
    typeof report.vulnerabilities !== "object" ||
    typeof report.metadata !== "object"
  ) {
    throw new Error(
      "not an npm audit v2 JSON report (registry or network failure?)",
    );
  }
  const found = new Map();
  for (const entry of Object.values(report.vulnerabilities)) {
    for (const via of entry.via ?? []) {
      if (typeof via !== "object" || via === null) continue; // inherited
      if (!BLOCKING_SEVERITIES.has(via.severity)) continue;
      const id = advisoryId(via.url);
      if (id === null) {
        throw new Error(`advisory without a GHSA URL on ${via.name}`);
      }
      found.set(id, { id, package: via.name, severity: via.severity });
    }
  }
  return [...found.values()].sort((a, b) => a.id.localeCompare(b.id));
}

/**
 * @returns {{ problems: string[], accepted: { id: string, package: string, severity: string }[] }}
 */
export function evaluate(report, exceptionsDoc, today = new Date()) {
  const problems = [];
  const accepted = [];
  const todayMs = Date.UTC(
    today.getUTCFullYear(),
    today.getUTCMonth(),
    today.getUTCDate(),
  );

  const list = exceptionsDoc?.exceptions;
  if (!Array.isArray(list)) {
    return { problems: ["exceptions file must have an `exceptions` array"], accepted };
  }

  const exceptions = new Map();
  for (const [i, ex] of list.entries()) {
    const who = `exception #${i + 1} (${ex?.advisory ?? "?"})`;
    if (!GHSA.test(ex?.advisory ?? "")) {
      problems.push(`${who}: advisory must be a GHSA id`);
      continue;
    }
    for (const field of ["package", "reason", "owner", "exit"]) {
      if (typeof ex[field] !== "string" || ex[field].trim() === "") {
        problems.push(`${who}: ${field} is required`);
      }
    }
    const added = parseDate(ex.added, "added", problems, who);
    const expires = parseDate(ex.expires, "expires", problems, who);
    if (added !== null && expires !== null) {
      if ((expires - added) / DAY_MS > MAX_EXCEPTION_DAYS) {
        problems.push(
          `${who}: exceptions may last at most ${MAX_EXCEPTION_DAYS} days — re-review instead of extending`,
        );
      }
      if (expires < todayMs) {
        problems.push(`${who}: expired on ${ex.expires} — re-review it`);
      }
    }
    if (exceptions.has(ex.advisory)) {
      problems.push(`${who}: duplicate entry`);
    }
    exceptions.set(ex.advisory, ex);
  }

  let advisories;
  try {
    advisories = blockingAdvisories(report);
  } catch (err) {
    problems.push(err.message);
    return { problems, accepted };
  }

  const seen = new Set();
  for (const adv of advisories) {
    seen.add(adv.id);
    const ex = exceptions.get(adv.id);
    if (ex === undefined) {
      problems.push(
        `${adv.severity} ${adv.id} in ${adv.package} has no reviewed exception — upgrade it`,
      );
    } else if (ex.package !== adv.package) {
      problems.push(
        `${adv.id} is excepted for ${ex.package} but was reported in ${adv.package}`,
      );
    } else {
      accepted.push(adv);
    }
  }
  for (const id of exceptions.keys()) {
    if (!seen.has(id)) {
      problems.push(`stale exception ${id}: no longer reported — remove it`);
    }
  }
  return { problems, accepted };
}

function main(argv) {
  if (argv.length !== 2) {
    console.error("usage: npm-audit-gate.mjs <npm-audit.json> <exceptions.json>");
    return 2;
  }
  let report;
  let exceptionsDoc;
  try {
    report = JSON.parse(readFileSync(argv[0], "utf8"));
    exceptionsDoc = JSON.parse(readFileSync(argv[1], "utf8"));
  } catch (err) {
    console.error(`npm audit gate: cannot read input: ${err.message}`);
    return 1;
  }
  const { problems, accepted } = evaluate(report, exceptionsDoc);
  for (const adv of accepted) {
    console.log(
      `accepted (reviewed exception): ${adv.severity} ${adv.id} in ${adv.package}`,
    );
  }
  if (problems.length > 0) {
    for (const p of problems) console.error(`npm audit gate: ${p}`);
    return 1;
  }
  console.log("npm audit gate: no unreviewed high/critical advisories");
  return 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2));
}
