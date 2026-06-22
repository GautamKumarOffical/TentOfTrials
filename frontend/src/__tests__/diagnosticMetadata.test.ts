import { describe, it, expect } from "vitest";
import {
  compareDiagnosticMetadata,
  parseDiagnosticMetadata,
  DiagnosticMetadata,
} from "../utils/diagnosticMetadata";

const createBaselineMetadata = (): DiagnosticMetadata => ({
  generated_at: "2026-06-16T10:00:00Z",
  commit: "aaa11111",
  diagnostic_logd: "diagnostic/build-aaa11111.logd",
  diagnostic_logd_error: null,
  chunked: false,
  chunk_size_bytes: null,
  password: "testpassword123",
  decrypt_command: "encryptly unpack diagnostic/build-aaa11111.logd <outdir> --password testpassword123",
  total_modules: 4,
  passed: 3,
  failed: 1,
  modules: [
    { name: "frailbox", status: "PASS", elapsed_seconds: 1.5, artifact: "frailbox.bin", output: "OK" },
    { name: "market", status: "PASS", elapsed_seconds: 2.1, artifact: "market.bin", output: "OK" },
    { name: "backend", status: "FAIL", elapsed_seconds: 0.5, artifact: null, output: "Compilation error" },
    { name: "tools", status: "PASS", elapsed_seconds: 0.8, artifact: "tools.bin", output: "OK" },
  ],
  pr_note: "Include this JSON diagnostic report in your PR.",
});

const createCandidateMetadata = (): DiagnosticMetadata => ({
  generated_at: "2026-06-16T12:00:00Z",
  commit: "bbb22222",
  diagnostic_logd: "diagnostic/build-bbb22222.logd",
  diagnostic_logd_error: null,
  chunked: false,
  chunk_size_bytes: null,
  password: "newpassword456",
  decrypt_command: "encryptly unpack diagnostic/build-bbb22222.logd <outdir> --password newpassword456",
  total_modules: 5,
  passed: 4,
  failed: 1,
  modules: [
    { name: "frailbox", status: "PASS", elapsed_seconds: 1.2, artifact: "frailbox.bin", output: "OK" },
    { name: "market", status: "PASS", elapsed_seconds: 2.3, artifact: "market-v2.bin", output: "OK" },
    { name: "backend", status: "PASS", elapsed_seconds: 1.0, artifact: "backend.bin", output: "Fixed compilation" },
    { name: "tools", status: "PASS", elapsed_seconds: 0.9, artifact: "tools.bin", output: "OK" },
    { name: "newmodule", status: "FAIL", elapsed_seconds: 0.2, artifact: null, output: "Missing dependency" },
  ],
  pr_note: "Include this JSON diagnostic report in your PR.",
});

describe("parseDiagnosticMetadata", () => {
  it("should parse valid JSON string", () => {
    const metadata = createBaselineMetadata();
    const result = parseDiagnosticMetadata(JSON.stringify(metadata));
    expect(result).toEqual(metadata);
  });

  it("should throw on invalid JSON", () => {
    expect(() => parseDiagnosticMetadata("invalid json")).toThrow();
  });

  it("should throw on missing commit field", () => {
    const invalid = { modules: [] };
    expect(() => parseDiagnosticMetadata(JSON.stringify(invalid))).toThrow("Invalid diagnostic metadata format");
  });

  it("should throw on missing modules array", () => {
    const invalid = { commit: "test" };
    expect(() => parseDiagnosticMetadata(JSON.stringify(invalid))).toThrow("Invalid diagnostic metadata format");
  });

  it("should throw on invalid module status", () => {
    const invalid = {
      commit: "test",
      modules: [{ name: "test", status: "INVALID" }],
    };
    expect(() => parseDiagnosticMetadata(JSON.stringify(invalid))).toThrow("Invalid module entry");
  });
});

describe("compareDiagnosticMetadata", () => {
  it("should detect recovered modules (FAIL -> PASS)", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    const recovered = comparison.moduleChanges.filter((c) => c.type === "recovered");
    expect(recovered).toHaveLength(1);
    expect(recovered[0].moduleName).toBe("backend");
    expect(recovered[0].baseline?.status).toBe("FAIL");
    expect(recovered[0].candidate?.status).toBe("PASS");
  });

  it("should detect newly failed modules (PASS -> FAIL)", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    const newlyFailed = comparison.moduleChanges.filter((c) => c.type === "newly_failed");
    expect(newlyFailed).toHaveLength(1);
    expect(newlyFailed[0].moduleName).toBe("newmodule");
    expect(newlyFailed[0].baseline).toBeUndefined();
    expect(newlyFailed[0].candidate?.status).toBe("FAIL");
  });

  it("should detect added modules", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    const added = comparison.moduleChanges.filter((c) => c.type === "added");
    expect(added).toHaveLength(1);
    expect(added[0].moduleName).toBe("newmodule");
  });

  it("should detect changed modules (same status, different output)", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    const changed = comparison.moduleChanges.filter((c) => c.type === "changed");
    expect(changed).toHaveLength(1);
    expect(changed[0].moduleName).toBe("market");
  });

  it("should detect artifact changes", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    expect(comparison.artifactChanges).toHaveLength(1);
    expect(comparison.artifactChanges[0].moduleName).toBe("market");
    expect(comparison.artifactChanges[0].baselineArtifact).toBe("market.bin");
    expect(comparison.artifactChanges[0].candidateArtifact).toBe("market-v2.bin");
  });

  it("should calculate correct summary", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    expect(comparison.summary).toEqual({
      totalBaselineModules: 4,
      totalCandidateModules: 5,
      added: 1,
      removed: 0,
      changed: 1,
      recovered: 1,
      newlyFailed: 1,
      unchanged: 1,
    });
  });

  it("should handle identical builds with no changes", () => {
    const metadata = createBaselineMetadata();
    const comparison = compareDiagnosticMetadata(metadata, metadata);

    expect(comparison.moduleChanges).toHaveLength(0);
    expect(comparison.artifactChanges).toHaveLength(0);
    expect(comparison.summary.unchanged).toBe(4);
  });

  it("should detect removed modules", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    candidate.modules = candidate.modules.filter((m) => m.name !== "tools");

    const comparison = compareDiagnosticMetadata(baseline, candidate);
    const removed = comparison.moduleChanges.filter((c) => c.type === "removed");
    expect(removed).toHaveLength(1);
    expect(removed[0].moduleName).toBe("tools");
  });

  it("should sort module changes alphabetically", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    const names = comparison.moduleChanges.map((c) => c.moduleName);
    const sortedNames = [...names].sort();
    expect(names).toEqual(sortedNames);
  });

  it("should not display encrypted log contents", () => {
    const baseline = createBaselineMetadata();
    const candidate = createCandidateMetadata();
    const comparison = compareDiagnosticMetadata(baseline, candidate);

    const jsonStr = JSON.stringify(comparison);
    expect(jsonStr).not.toContain("testpassword123");
    expect(jsonStr).not.toContain("newpassword456");
  });
});
