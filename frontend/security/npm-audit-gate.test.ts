import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

import { evaluate } from "./npm-audit-gate.mjs";

const TODAY = new Date("2026-10-08T12:00:00Z");

function advisory(name: string, id: string, severity = "high") {
  return {
    source: 1,
    name,
    title: "t",
    url: `https://github.com/advisories/${id}`,
    severity,
    range: "*",
  };
}

function report(vulns: Record<string, { severity: string; via: unknown[] }>) {
  return { auditReportVersion: 2, vulnerabilities: vulns, metadata: {} };
}

const BRACES = report({
  braces: { severity: "high", via: [advisory("braces", "GHSA-vfj7-8cjw-p6xm")] },
  micromatch: { severity: "high", via: ["braces"] },
});

function exception(overrides: Record<string, string> = {}) {
  return {
    advisory: "GHSA-vfj7-8cjw-p6xm",
    package: "braces",
    reason: "no fix",
    owner: "o",
    exit: "upgrade",
    added: "2026-10-08",
    expires: "2026-11-08",
    ...overrides,
  };
}

describe("npm audit gate", () => {
  it("accepts a reviewed, unexpired exception and ignores inherited entries", () => {
    const r = evaluate(BRACES, { exceptions: [exception()] }, TODAY);
    expect(r.problems).toEqual([]);
    expect(r.accepted.map((a) => a.id)).toEqual(["GHSA-vfj7-8cjw-p6xm"]);
  });

  it("fails an unreviewed high or critical advisory", () => {
    const r = evaluate(
      report({
        next: { severity: "critical", via: [advisory("next", "GHSA-vcvr-r3jv-pc5j", "critical")] },
      }),
      { exceptions: [] },
      TODAY,
    );
    expect(r.problems.join("\n")).toMatch(/critical GHSA-vcvr-r3jv-pc5j in next/);
  });

  it("does not block moderate advisories", () => {
    const r = evaluate(
      report({ x: { severity: "moderate", via: [advisory("x", "GHSA-rj75-hqrm-r3gf", "moderate")] } }),
      { exceptions: [] },
      TODAY,
    );
    expect(r.problems).toEqual([]);
  });

  it("fails an expired exception", () => {
    const r = evaluate(
      BRACES,
      { exceptions: [exception({ added: "2026-09-01", expires: "2026-10-07" })] },
      TODAY,
    );
    expect(r.problems.join("\n")).toMatch(/expired/);
  });

  it("refuses exceptions longer than 90 days", () => {
    const r = evaluate(BRACES, { exceptions: [exception({ expires: "2027-03-01" })] }, TODAY);
    expect(r.problems.join("\n")).toMatch(/at most 90 days/);
  });

  it("fails a stale exception", () => {
    const r = evaluate(report({}), { exceptions: [exception()] }, TODAY);
    expect(r.problems.join("\n")).toMatch(/stale exception/);
  });

  it("does not let an exception cover a different package", () => {
    const r = evaluate(BRACES, { exceptions: [exception({ package: "micromatch" })] }, TODAY);
    expect(r.problems.join("\n")).toMatch(/excepted for micromatch/);
  });

  it("never treats an error report as clean", () => {
    const r = evaluate({ error: { code: "ENOTFOUND" } }, { exceptions: [] }, TODAY);
    expect(r.problems.join("\n")).toMatch(/not an npm audit v2 JSON report/);
  });

  it("checked-in exceptions file is well-formed and time-boxed", () => {
    const doc = JSON.parse(
      readFileSync(path.join(import.meta.dirname, "npm-audit-exceptions.json"), "utf8"),
    );
    // Evaluate against a report that contains exactly the excepted advisories,
    // on the day each was added: only schema/duration problems can surface.
    for (const ex of doc.exceptions) {
      const r = evaluate(
        report({ [ex.package]: { severity: "high", via: [advisory(ex.package, ex.advisory)] } }),
        { exceptions: [ex] },
        new Date(`${ex.added}T00:00:00Z`),
      );
      expect(r.problems).toEqual([]);
    }
  });
});
