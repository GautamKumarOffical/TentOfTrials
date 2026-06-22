import React, { useState, useCallback } from "react";
import {
  compareDiagnosticMetadata,
  parseDiagnosticMetadata,
  formatTimestamp,
  getChangeTypeLabel,
  getChangeTypeColor,
  getStatusColor,
  DiagnosticMetadata,
  DiagnosticComparison,
  ModuleChange,
  ArtifactChange,
} from "../utils/diagnosticMetadata";

interface ParsedFile {
  name: string;
  metadata: DiagnosticMetadata;
}

const DiagnosticCompare: React.FC = () => {
  const [baselineFile, setBaselineFile] = useState<ParsedFile | null>(null);
  const [candidateFile, setCandidateFile] = useState<ParsedFile | null>(null);
  const [comparison, setComparison] = useState<DiagnosticComparison | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleFileUpload = useCallback(
    (file: File, type: "baseline" | "candidate") => {
      setError(null);
      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const content = e.target?.result as string;
          const metadata = parseDiagnosticMetadata(content);
          const parsed: ParsedFile = { name: file.name, metadata };
          if (type === "baseline") {
            setBaselineFile(parsed);
          } else {
            setCandidateFile(parsed);
          }
        } catch (err) {
          setError(`Failed to parse ${type} file: ${err instanceof Error ? err.message : "Unknown error"}`);
        }
      };
      reader.readAsText(file);
    },
    []
  );

  const runComparison = useCallback(() => {
    if (!baselineFile || !candidateFile) {
      setError("Please upload both baseline and candidate files");
      return;
    }
    try {
      const result = compareDiagnosticMetadata(baselineFile.metadata, candidateFile.metadata);
      setComparison(result);
      setError(null);
    } catch (err) {
      setError(`Comparison failed: ${err instanceof Error ? err.message : "Unknown error"}`);
    }
  }, [baselineFile, candidateFile]);

  const renderModuleChange = (change: ModuleChange) => (
    <tr key={change.moduleName} className="border-b border-gray-200 hover:bg-gray-50">
      <td className="px-4 py-3 font-medium">{change.moduleName}</td>
      <td className="px-4 py-3">
        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getChangeTypeColor(change.type)}`}>
          {getChangeTypeLabel(change.type)}
        </span>
      </td>
      <td className="px-4 py-3">
        {change.baseline ? (
          <span className={getStatusColor(change.baseline.status)}>
            {change.baseline.status}
          </span>
        ) : (
          <span className="text-gray-400">-</span>
        )}
      </td>
      <td className="px-4 py-3">
        {change.candidate ? (
          <span className={getStatusColor(change.candidate.status)}>
            {change.candidate.status}
          </span>
        ) : (
          <span className="text-gray-400">-</span>
        )}
      </td>
      <td className="px-4 py-3 text-sm text-gray-600 max-w-xs truncate">
        {change.candidate?.output || change.baseline?.output || "-"}
      </td>
    </tr>
  );

  const renderArtifactChange = (change: ArtifactChange) => (
    <tr key={change.moduleName} className="border-b border-gray-200 hover:bg-gray-50">
      <td className="px-4 py-3 font-medium">{change.moduleName}</td>
      <td className="px-4 py-3 text-sm text-gray-600">{change.baselineArtifact || "(none)"}</td>
      <td className="px-4 py-3 text-sm text-gray-600">{change.candidateArtifact || "(none)"}</td>
    </tr>
  );

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold mb-6">Diagnostic Metadata Compare</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        <div className="border border-gray-300 rounded-lg p-4">
          <h2 className="text-lg font-semibold mb-3">Baseline (Previous Build)</h2>
          <input
            type="file"
            accept=".json"
            onChange={(e) => e.target.files?.[0] && handleFileUpload(e.target.files[0], "baseline")}
            className="block w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
          />
          {baselineFile && (
            <div className="mt-3 p-3 bg-gray-50 rounded">
              <p className="text-sm"><strong>File:</strong> {baselineFile.name}</p>
              <p className="text-sm"><strong>Commit:</strong> {baselineFile.metadata.commit}</p>
              <p className="text-sm"><strong>Generated:</strong> {formatTimestamp(baselineFile.metadata.generated_at)}</p>
              <p className="text-sm"><strong>Modules:</strong> {baselineFile.metadata.total_modules} (Passed: {baselineFile.metadata.passed}, Failed: {baselineFile.metadata.failed})</p>
            </div>
          )}
        </div>

        <div className="border border-gray-300 rounded-lg p-4">
          <h2 className="text-lg font-semibold mb-3">Candidate (Current Build)</h2>
          <input
            type="file"
            accept=".json"
            onChange={(e) => e.target.files?.[0] && handleFileUpload(e.target.files[0], "candidate")}
            className="block w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
          />
          {candidateFile && (
            <div className="mt-3 p-3 bg-gray-50 rounded">
              <p className="text-sm"><strong>File:</strong> {candidateFile.name}</p>
              <p className="text-sm"><strong>Commit:</strong> {candidateFile.metadata.commit}</p>
              <p className="text-sm"><strong>Generated:</strong> {formatTimestamp(candidateFile.metadata.generated_at)}</p>
              <p className="text-sm"><strong>Modules:</strong> {candidateFile.metadata.total_modules} (Passed: {candidateFile.metadata.passed}, Failed: {candidateFile.metadata.failed})</p>
            </div>
          )}
        </div>
      </div>

      <div className="mb-6">
        <button
          onClick={runComparison}
          disabled={!baselineFile || !candidateFile}
          className="px-6 py-2 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed"
        >
          Compare Builds
        </button>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">
          {error}
        </div>
      )}

      {comparison && (
        <div className="space-y-8">
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <h2 className="text-xl font-semibold mb-4">Summary</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="text-center p-4 bg-blue-50 rounded-lg">
                <div className="text-2xl font-bold text-blue-600">{comparison.summary.added}</div>
                <div className="text-sm text-gray-600">Added</div>
              </div>
              <div className="text-center p-4 bg-gray-50 rounded-lg">
                <div className="text-2xl font-bold text-gray-600">{comparison.summary.removed}</div>
                <div className="text-sm text-gray-600">Removed</div>
              </div>
              <div className="text-center p-4 bg-green-50 rounded-lg">
                <div className="text-2xl font-bold text-green-600">{comparison.summary.recovered}</div>
                <div className="text-sm text-gray-600">Recovered</div>
              </div>
              <div className="text-center p-4 bg-red-50 rounded-lg">
                <div className="text-2xl font-bold text-red-600">{comparison.summary.newlyFailed}</div>
                <div className="text-sm text-gray-600">Newly Failed</div>
              </div>
            </div>
            <div className="mt-4 text-sm text-gray-600">
              <p>Baseline: {comparison.baselineCommit} ({formatTimestamp(comparison.baselineGeneratedAt)})</p>
              <p>Candidate: {comparison.candidateCommit} ({formatTimestamp(comparison.candidateGeneratedAt)})</p>
            </div>
          </div>

          {comparison.moduleChanges.length > 0 && (
            <div className="bg-white border border-gray-200 rounded-lg p-6">
              <h2 className="text-xl font-semibold mb-4">Module Changes</h2>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Module</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Change Type</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Baseline</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Candidate</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Output</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {comparison.moduleChanges.map(renderModuleChange)}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {comparison.artifactChanges.length > 0 && (
            <div className="bg-white border border-gray-200 rounded-lg p-6">
              <h2 className="text-xl font-semibold mb-4">Artifact Changes</h2>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Module</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Baseline Artifact</th>
                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Candidate Artifact</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {comparison.artifactChanges.map(renderArtifactChange)}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {comparison.moduleChanges.length === 0 && comparison.artifactChanges.length === 0 && (
            <div className="bg-green-50 border border-green-200 rounded-lg p-6 text-center">
              <p className="text-green-700 font-semibold">No changes detected between builds</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default DiagnosticCompare;
