//! Health check endpoint for service monitoring.
//!
//! Provides a `GET /health` endpoint that returns service health information
//! including status, timestamp, version, and uptime.

use axum::{Json, response::IntoResponse};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::time::Instant;

use once_cell::sync::OnceCell;

/// Global startup time for uptime calculation.
static STARTUP_TIME: OnceCell<Instant> = OnceCell::new();

/// Initializes the startup time. Should be called once at application start.
pub fn init_startup_time() {
    let _ = STARTUP_TIME.set(Instant::now());
}

/// Returns the uptime in seconds since the service started.
fn get_uptime_seconds() -> u64 {
    STARTUP_TIME
        .get()
        .map(|start| start.elapsed().as_secs())
        .unwrap_or(0)
}

/// Health response payload.
///
/// Returned by the `GET /health` endpoint to provide service health information.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HealthResponse {
    /// The current health status of the service.
    pub status: String,
    
    /// The current UTC timestamp in ISO 8601 format.
    pub timestamp: String,
    
    /// The service version from Cargo.toml.
    pub version: String,
    
    /// The number of seconds since the service started.
    pub uptime_seconds: u64,
}

impl HealthResponse {
    /// Creates a new health response with current service state.
    pub fn new() -> Self {
        Self {
            status: "healthy".to_string(),
            timestamp: Utc::now().to_rfc3339(),
            version: env!("CARGO_PKG_VERSION").to_string(),
            uptime_seconds: get_uptime_seconds(),
        }
    }
}

impl Default for HealthResponse {
    fn default() -> Self {
        Self::new()
    }
}

/// Handler for the `GET /health` endpoint.
///
/// Returns a JSON response containing:
/// - `status`: Service health status ("healthy")
/// - `timestamp`: Current UTC timestamp in ISO 8601 format
/// - `version`: Service version from Cargo.toml
/// - `uptime_seconds`: Seconds since service started
///
/// # Example Response
///
/// ```json
/// {
///     "status": "healthy",
///     "timestamp": "2024-01-15T12:00:00Z",
///     "version": "0.1.0",
///     "uptime_seconds": 3600
/// }
/// ```
pub async fn health_handler() -> impl IntoResponse {
    Json(HealthResponse::new())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_health_response_new() {
        init_startup_time();
        let response = HealthResponse::new();
        
        assert_eq!(response.status, "healthy");
        assert_eq!(response.version, env!("CARGO_PKG_VERSION"));
        assert!(response.timestamp.len() > 0);
    }

    #[test]
    fn test_health_response_serializes() {
        init_startup_time();
        let response = HealthResponse::new();
        let json = serde_json::to_string(&response).unwrap();
        
        assert!(json.contains("\"status\":\"healthy\""));
        assert!(json.contains("\"version\""));
        assert!(json.contains("\"timestamp\""));
        assert!(json.contains("\"uptime_seconds\""));
    }

    #[test]
    fn test_health_response_deserializes() {
        let json = r#"{
            "status": "healthy",
            "timestamp": "2024-01-15T12:00:00Z",
            "version": "0.1.0",
            "uptime_seconds": 3600
        }"#;
        
        let response: HealthResponse = serde_json::from_str(json).unwrap();
        
        assert_eq!(response.status, "healthy");
        assert_eq!(response.timestamp, "2024-01-15T12:00:00Z");
        assert_eq!(response.version, "0.1.0");
        assert_eq!(response.uptime_seconds, 3600);
    }
}
