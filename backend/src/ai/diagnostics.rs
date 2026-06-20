//! # AI Diagnostics Subsystem  -  Intelligent System Health Scanning
//!
//! This module provides scaffolding for AI-powered diagnostics scanning across
//! the Tent of Trials backend. It enables automated health analysis, anomaly
//! detection, and predictive diagnostics using the inference and embeddings
//! subsystems.
//!
//! ## Key Components
//!
//! - `DiagnosticsScanner`  -  Main scanner that coordinates diagnostic runs
//! - `DiagnosticReport`  -  Structured output from diagnostic scans
//! - `AnomalyDetector`  -  Identifies unusual patterns in telemetry data
//! - `HealthAnalyzer`  -  Evaluates system component health scores

use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};

use serde::{Deserialize, Serialize};
use tokio::sync::RwLock;
use tracing::{debug, error, info, warn};

use super::inference::{InferenceConfig, InferenceResult, Message, MessageRole};
use super::AiOrchestrator;

// ---------------------------------------------------------------------------
// Constants  -  Diagnostics Configuration
// ---------------------------------------------------------------------------

/// Default interval between diagnostic scans (in seconds).
const DEFAULT_SCAN_INTERVAL_SECS: u64 = 60;

/// Maximum number of historical reports to retain in memory.
const MAX_REPORT_HISTORY: usize = 100;

/// Health score thresholds.
const HEALTH_SCORE_CRITICAL: f64 = 0.3;
const HEALTH_SCORE_WARNING: f64 = 0.6;
const HEALTH_SCORE_HEALTHY: f64 = 0.8;

// ---------------------------------------------------------------------------
// Types  -  Diagnostic Core Structures
// ---------------------------------------------------------------------------

/// Severity level for diagnostic findings.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Severity {
    Info,
    Warning,
    Critical,
    Emergency,
}

impl std::fmt::Display for Severity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Severity::Info => write!(f, "INFO"),
            Severity::Warning => write!(f, "WARNING"),
            Severity::Critical => write!(f, "CRITICAL"),
            Severity::Emergency => write!(f, "EMERGENCY"),
        }
    }
}

/// A single diagnostic finding.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiagnosticFinding {
    pub component: String,
    pub severity: Severity,
    pub message: String,
    pub metric_name: String,
    pub metric_value: f64,
    pub threshold: f64,
    pub recommendation: String,
}

/// The result of a single diagnostic scan.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiagnosticReport {
    pub scan_id: String,
    pub timestamp: i64,
    pub overall_health: f64,
    pub findings: Vec<DiagnosticFinding>,
    pub components_scanned: Vec<String>,
    pub scan_duration_ms: u64,
    pub ai_analysis: Option<String>,
}

impl DiagnosticReport {
    /// Creates a new empty diagnostic report.
    pub fn new(scan_id: impl Into<String>) -> Self {
        Self {
            scan_id: scan_id.into(),
            timestamp: chrono::Utc::now().timestamp(),
            overall_health: 1.0,
            findings: Vec::new(),
            components_scanned: Vec::new(),
            scan_duration_ms: 0,
            ai_analysis: None,
        }
    }

    /// Returns the count of findings by severity.
    pub fn findings_by_severity(&self) -> HashMap<Severity, usize> {
        let mut counts = HashMap::new();
        for finding in &self.findings {
            *counts.entry(finding.severity.clone()).or_insert(0) += 1;
        }
        counts
    }

    /// Returns true if any findings are critical or emergency.
    pub fn has_critical_findings(&self) -> bool {
        self.findings
            .iter()
            .any(|f| f.severity == Severity::Critical || f.severity == Severity::Emergency)
    }
}

/// Health status for a single component.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComponentHealth {
    pub name: String,
    pub score: f64,
    pub latency_ms: f64,
    pub error_rate: f64,
    pub last_checked: i64,
}

/// Configuration for the diagnostics scanner.
#[derive(Debug, Clone)]
pub struct DiagnosticsConfig {
    pub scan_interval: Duration,
    pub auto_scan_enabled: bool,
    pub components: Vec<String>,
    pub thresholds: HashMap<String, f64>,
}

impl Default for DiagnosticsConfig {
    fn default() -> Self {
        let mut thresholds = HashMap::new();
        thresholds.insert("error_rate".to_string(), 0.05);
        thresholds.insert("latency_ms".to_string(), 500.0);
        thresholds.insert("cpu_usage".to_string(), 0.85);
        thresholds.insert("memory_usage".to_string(), 0.90);

        Self {
            scan_interval: Duration::from_secs(DEFAULT_SCAN_INTERVAL_SECS),
            auto_scan_enabled: true,
            components: vec![
                "discovery".to_string(),
                "messaging".to_string(),
                "registry".to_string(),
            ],
            thresholds,
        }
    }
}

// ---------------------------------------------------------------------------
// Anomaly Detector
// ---------------------------------------------------------------------------

/// Detects anomalies in metric time series data using statistical methods.
pub struct AnomalyDetector {
    history: RwLock<Vec<(String, f64, Instant)>>,
    window_size: usize,
}

impl AnomalyDetector {
    pub fn new(window_size: Option<usize>) -> Self {
        Self {
            history: RwLock::new(Vec::new()),
            window_size: window_size.unwrap_or(100),
        }
    }

    /// Records a metric value for anomaly detection.
    pub async fn record(&self, metric_name: &str, value: f64) {
        let mut history = self.history.write().await;
        history.push((metric_name.to_string(), value, Instant::now()));
        if history.len() > self.window_size {
            let excess = history.len() - self.window_size;
            history.drain(0..excess);
        }
    }

    /// Checks if a value is anomalous based on historical data.
    pub async fn is_anomalous(&self, metric_name: &str, value: f64) -> bool {
        let history = self.history.read().await;
        let values: Vec<f64> = history
            .iter()
            .filter(|(name, _, _)| name == metric_name)
            .map(|(_, v, _)| *v)
            .collect();

        if values.len() < 10 {
            return false;
        }

        let mean = values.iter().sum::<f64>() / values.len() as f64;
        let variance = values.iter().map(|v| (v - mean).powi(2)).sum::<f64>() / values.len() as f64;
        let std_dev = variance.sqrt();

        if std_dev == 0.0 {
            return false;
        }

        let z_score = (value - mean).abs() / std_dev;
        z_score > 3.0
    }
}

// ---------------------------------------------------------------------------
// Diagnostics Scanner
// ---------------------------------------------------------------------------

/// The main diagnostics scanner that coordinates AI-powered system analysis.
pub struct DiagnosticsScanner {
    orchestrator: Arc<AiOrchestrator>,
    config: RwLock<DiagnosticsConfig>,
    reports: RwLock<Vec<DiagnosticReport>>,
    anomaly_detector: AnomalyDetector,
    is_running: Arc<RwLock<bool>>,
}

impl DiagnosticsScanner {
    /// Creates a new diagnostics scanner.
    pub fn new(orchestrator: Arc<AiOrchestrator>, config: Option<DiagnosticsConfig>) -> Self {
        Self {
            orchestrator,
            config: RwLock::new(config.unwrap_or_default()),
            reports: RwLock::new(Vec::with_capacity(MAX_REPORT_HISTORY)),
            anomaly_detector: AnomalyDetector::new(None),
            is_running: Arc::new(RwLock::new(false)),
        }
    }

    /// Runs a single diagnostic scan across all configured components.
    pub async fn run_scan(&self) -> DiagnosticReport {
        let scan_start = Instant::now();
        let scan_id = uuid::Uuid::new_v4().to_string();
        let mut report = DiagnosticReport::new(&scan_id);

        let config = self.config.read().await;
        let components = config.components.clone();
        let thresholds = config.thresholds.clone();
        drop(config);

        info!("diagnostics: starting scan {}", scan_id);

        for component in &components {
            report.components_scanned.push(component.clone());

            let finding = self.scan_component(component, &thresholds).await;
            if let Some(f) = finding {
                report.findings.push(f);
            }

            self.anomaly_detector
                .record(&format!("{}.health", component), report.overall_health)
                .await;
        }

        report.scan_duration_ms = scan_start.elapsed().as_millis() as u64;

        if !report.findings.is_empty() {
            report.overall_health = 1.0 - (report.findings.len() as f64 * 0.1).min(0.7);
        }

        let mut reports = self.reports.write().await;
        reports.push(report.clone());
        if reports.len() > MAX_REPORT_HISTORY {
            let excess = reports.len() - MAX_REPORT_HISTORY;
            reports.drain(0..excess);
        }

        info!(
            "diagnostics: scan {} complete (health: {:.2}, findings: {})",
            scan_id,
            report.overall_health,
            report.findings.len()
        );

        report
    }

    /// Scans a single component and returns an optional finding.
    async fn scan_component(
        &self,
        component: &str,
        thresholds: &HashMap<String, f64>,
    ) -> Option<DiagnosticFinding> {
        debug!("diagnostics: scanning component '{}'", component);

        let node_states = self.orchestrator.node_state_summary().await;

        if node_states.is_empty() {
            return Some(DiagnosticFinding {
                component: component.to_string(),
                severity: Severity::Warning,
                message: format!("No nodes registered for component '{}'", component),
                metric_name: "node_count".to_string(),
                metric_value: 0.0,
                threshold: 1.0,
                recommendation: "Ensure service discovery is running and nodes are registered.".to_string(),
            });
        }

        let avg_latency: f64 = node_states.iter().map(|s| s.ewma_latency_ms).sum::<f64>()
            / node_states.len() as f64;
        let latency_threshold = thresholds.get("latency_ms").copied().unwrap_or(500.0);

        if avg_latency > latency_threshold {
            return Some(DiagnosticFinding {
                component: component.to_string(),
                severity: Severity::Warning,
                message: format!(
                    "Average latency {:.1}ms exceeds threshold {:.1}ms",
                    avg_latency, latency_threshold
                ),
                metric_name: "latency_ms".to_string(),
                metric_value: avg_latency,
                threshold: latency_threshold,
                recommendation: "Consider scaling up node pool or investigating network issues.".to_string(),
            });
        }

        None
    }

    /// Starts the automatic diagnostic scan loop.
    pub async fn start_auto_scan(&self) {
        let config = self.config.read().await;
        let interval = config.scan_interval;
        drop(config);

        let mut is_running = self.is_running.write().await;
        *is_running = true;
        drop(is_running);

        info!("diagnostics: auto-scan started with interval {:?}", interval);
    }

    /// Stops the automatic scan loop.
    pub async fn stop_auto_scan(&self) {
        *self.is_running.write().await = false;
        info!("diagnostics: auto-scan stopped");
    }

    /// Returns the most recent diagnostic report.
    pub async fn latest_report(&self) -> Option<DiagnosticReport> {
        let reports = self.reports.read().await;
        reports.last().cloned()
    }

    /// Returns all reports in the history.
    pub async fn report_history(&self) -> Vec<DiagnosticReport> {
        let reports = self.reports.read().await;
        reports.clone()
    }

    /// Updates the scanner configuration.
    pub async fn update_config(&self, config: DiagnosticsConfig) {
        *self.config.write().await = config;
        info!("diagnostics: configuration updated");
    }
}

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------

/// Creates and returns a diagnostics scanner wired into the AI orchestrator.
pub fn create_scanner(orchestrator: Arc<AiOrchestrator>) -> DiagnosticsScanner {
    DiagnosticsScanner::new(orchestrator, None)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_severity_ordering() {
        assert!(Severity::Info < Severity::Warning);
        assert!(Severity::Warning < Severity::Critical);
        assert!(Severity::Critical < Severity::Emergency);
    }

    #[test]
    fn test_diagnostic_report_findings_by_severity() {
        let mut report = DiagnosticReport::new("test-scan");
        report.findings.push(DiagnosticFinding {
            component: "test".to_string(),
            severity: Severity::Warning,
            message: "test warning".to_string(),
            metric_name: "test_metric".to_string(),
            metric_value: 1.0,
            threshold: 0.5,
            recommendation: "fix it".to_string(),
        });
        report.findings.push(DiagnosticFinding {
            component: "test".to_string(),
            severity: Severity::Warning,
            message: "test warning 2".to_string(),
            metric_name: "test_metric".to_string(),
            metric_value: 1.0,
            threshold: 0.5,
            recommendation: "fix it".to_string(),
        });
        report.findings.push(DiagnosticFinding {
            component: "test".to_string(),
            severity: Severity::Critical,
            message: "test critical".to_string(),
            metric_name: "test_metric".to_string(),
            metric_value: 2.0,
            threshold: 0.5,
            recommendation: "fix it now".to_string(),
        });

        let counts = report.findings_by_severity();
        assert_eq!(counts.get(&Severity::Warning), Some(&2));
        assert_eq!(counts.get(&Severity::Critical), Some(&1));
    }

    #[test]
    fn test_diagnostic_report_has_critical() {
        let mut report = DiagnosticReport::new("test");
        assert!(!report.has_critical_findings());

        report.findings.push(DiagnosticFinding {
            component: "test".to_string(),
            severity: Severity::Critical,
            message: "bad".to_string(),
            metric_name: "x".to_string(),
            metric_value: 0.0,
            threshold: 0.0,
            recommendation: "y".to_string(),
        });
        assert!(report.has_critical_findings());
    }
}
