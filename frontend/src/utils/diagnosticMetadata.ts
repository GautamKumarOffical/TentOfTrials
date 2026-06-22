export interface DiagnosticModule {
  name: string;
  status: "PASS" | "FAIL";
  elapsed_seconds: number;
  artifact: string | null;
  output: string;
}

export interface DiagnosticMetadata {
  generated_at: string;
  commit: string;
  diagnostic_logd: string | string[] | null;
  diagnostic_logd_error: string | null;
  chunked: boolean;
  chunk_size_bytes: number | null;
  password: string | null;
  decrypt_command: string | null;
  total_modules: number;
  passed: number;
  failed: number;
  modules: DiagnosticModule[];
  pr_note: string;
}

export type ModuleChangeType = "added" | "removed" | "changed" | "recovered" | "newly_failed";

export interface ModuleChange {
  moduleName: string;
  type: ModuleChangeType;
  baseline?: DiagnosticModule;
  candidate?: DiagnosticModule;
}

export interface ArtifactChange {
  moduleName: string;
  baselineArtifact: string | null;
  candidateArtifact: string | null;
}

export interface DiagnosticComparison {
  baselineCommit: string;
  candidateCommit: string;
  baselineGeneratedAt: string;
  candidateGeneratedAt: string;
  moduleChanges: ModuleChange[];
  artifactChanges: ArtifactChange[];
  summary: {
    totalBaselineModules: number;
    totalCandidateModules: number;
    added: number;
    removed: number;
    changed: number;
    recovered: number;
    newlyFailed: number;
    unchanged: number;
  };
}

function compareModuleArtifacts(
  baseline: DiagnosticModule | undefined,
  candidate: DiagnosticModule | undefined
): boolean {
  const baselineArtifact = baseline?.artifact ?? null;
  const candidateArtifact = candidate?.artifact ?? null;
  return baselineArtifact !== candidateArtifact;
}

function getModuleByName(
  modules: DiagnosticModule[],
  name: string
): DiagnosticModule | undefined {
  return modules.find((m) => m.name === name);
}

export function compareDiagnosticMetadata(
  baseline: DiagnosticMetadata,
  candidate: DiagnosticMetadata
): DiagnosticComparison {
  const baselineModuleNames = new Set(baseline.modules.map((m) => m.name));
  const candidateModuleNames = new Set(candidate.modules.map((m) => m.name));

  const allModuleNames = new Set([...baselineModuleNames, ...candidateModuleNames]);
  const moduleChanges: ModuleChange[] = [];
  const artifactChanges: ArtifactChange[] = [];

  let added = 0;
  let removed = 0;
  let changed = 0;
  let recovered = 0;
  let newlyFailed = 0;
  let unchanged = 0;

  for (const moduleName of allModuleNames) {
    const baselineModule = getModuleByName(baseline.modules, moduleName);
    const candidateModule = getModuleByName(candidate.modules, moduleName);

    if (!baselineModule && candidateModule) {
      moduleChanges.push({
        moduleName,
        type: "added",
        candidate: candidateModule,
      });
      added++;
    } else if (baselineModule && !candidateModule) {
      moduleChanges.push({
        moduleName,
        type: "removed",
        baseline: baselineModule,
      });
      removed++;
    } else if (baselineModule && candidateModule) {
      const statusChanged = baselineModule.status !== candidateModule.status;
      const outputChanged = baselineModule.output !== candidateModule.output;
      const elapsedChanged = baselineModule.elapsed_seconds !== candidateModule.elapsed_seconds;

      if (statusChanged) {
        if (baselineModule.status === "FAIL" && candidateModule.status === "PASS") {
          moduleChanges.push({
            moduleName,
            type: "recovered",
            baseline: baselineModule,
            candidate: candidateModule,
          });
          recovered++;
        } else if (baselineModule.status === "PASS" && candidateModule.status === "FAIL") {
          moduleChanges.push({
            moduleName,
            type: "newly_failed",
            baseline: baselineModule,
            candidate: candidateModule,
          });
          newlyFailed++;
        } else {
          moduleChanges.push({
            moduleName,
            type: "changed",
            baseline: baselineModule,
            candidate: candidateModule,
          });
          changed++;
        }
      } else if (outputChanged || elapsedChanged) {
        moduleChanges.push({
          moduleName,
          type: "changed",
          baseline: baselineModule,
          candidate: candidateModule,
        });
        changed++;
      } else {
        unchanged++;
      }

      if (compareModuleArtifacts(baselineModule, candidateModule)) {
        artifactChanges.push({
          moduleName,
          baselineArtifact: baselineModule.artifact,
          candidateArtifact: candidateModule.artifact,
        });
      }
    }
  }

  moduleChanges.sort((a, b) => a.moduleName.localeCompare(b.moduleName));
  artifactChanges.sort((a, b) => a.moduleName.localeCompare(b.moduleName));

  return {
    baselineCommit: baseline.commit,
    candidateCommit: candidate.commit,
    baselineGeneratedAt: baseline.generated_at,
    candidateGeneratedAt: candidate.generated_at,
    moduleChanges,
    artifactChanges,
    summary: {
      totalBaselineModules: baseline.total_modules,
      totalCandidateModules: candidate.total_modules,
      added,
      removed,
      changed,
      recovered,
      newlyFailed,
      unchanged,
    },
  };
}

export function parseDiagnosticMetadata(jsonString: string): DiagnosticMetadata {
  const parsed = JSON.parse(jsonString);

  if (
    typeof parsed !== "object" ||
    parsed === null ||
    typeof parsed.commit !== "string" ||
    !Array.isArray(parsed.modules)
  ) {
    throw new Error("Invalid diagnostic metadata format");
  }

  for (const module of parsed.modules) {
    if (
      typeof module.name !== "string" ||
      (module.status !== "PASS" && module.status !== "FAIL")
    ) {
      throw new Error(`Invalid module entry: ${JSON.stringify(module)}`);
    }
  }

  return parsed as DiagnosticMetadata;
}

export function formatTimestamp(isoString: string): string {
  try {
    const date = new Date(isoString);
    return date.toLocaleString();
  } catch {
    return isoString;
  }
}

export function getStatusColor(status: "PASS" | "FAIL"): string {
  return status === "PASS" ? "text-green-600" : "text-red-600";
}

export function getChangeTypeLabel(type: ModuleChangeType): string {
  switch (type) {
    case "added":
      return "Added";
    case "removed":
      return "Removed";
    case "changed":
      return "Changed";
    case "recovered":
      return "Recovered";
    case "newly_failed":
      return "Newly Failed";
    default:
      return type;
  }
}

export function getChangeTypeColor(type: ModuleChangeType): string {
  switch (type) {
    case "added":
      return "text-blue-600 bg-blue-50";
    case "removed":
      return "text-gray-600 bg-gray-50";
    case "changed":
      return "text-yellow-600 bg-yellow-50";
    case "recovered":
      return "text-green-600 bg-green-50";
    case "newly_failed":
      return "text-red-600 bg-red-50";
    default:
      return "text-gray-600 bg-gray-50";
  }
}
