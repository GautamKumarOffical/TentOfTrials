use serde::Serialize;
use std::time::Instant;

#[derive(Debug, Clone, Serialize)]
pub struct HealthResponse {
    pub status: String,
    pub version: String,
    pub commit: String,
    pub uptime_seconds: u64,
    pub features: Vec<String>,
}

pub struct HealthService {
    start_time: Instant,
    commit: String,
}

impl HealthService {
    pub fn new() -> Self {
        let commit = std::env::var("GIT_COMMIT").unwrap_or_else(|_| "unknown".into());
        Self {
            start_time: Instant::now(),
            commit,
        }
    }

    pub fn get_health(&self) -> HealthResponse {
        HealthResponse {
            status: "ok".into(),
            version: crate::VERSION.into(),
            commit: self.commit.clone(),
            uptime_seconds: self.start_time.elapsed().as_secs(),
            features: self.get_enabled_features(),
        }
    }

    fn get_enabled_features(&self) -> Vec<String> {
        let mut features = Vec::new();
        if std::env::var("TOT_ENABLE_EXPERIMENTAL").is_ok() {
            features.push("experimental".into());
        }
        features
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    #[test]
    fn test_health_response_has_required_fields() {
        let service = HealthService::new();
        let response = service.get_health();
        assert_eq!(response.status, "ok");
        assert!(!response.version.is_empty());
        assert!(!response.commit.is_empty());
        assert!(response.uptime_seconds < 10);
    }

    #[test]
    fn test_health_response_json_shape() {
        let service = HealthService::new();
        let response = service.get_health();
        let json = serde_json::to_string(&response).unwrap();
        let parsed: serde_json::Value = serde_json::from_str(&json).unwrap();
        assert!(parsed.get("status").is_some());
        assert!(parsed.get("version").is_some());
        assert!(parsed.get("commit").is_some());
        assert!(parsed.get("uptime_seconds").is_some());
        assert!(parsed.get("features").is_some());
    }

    #[test]
    fn test_uptime_increases() {
        let service = HealthService::new();
        let r1 = service.get_health();
        std::thread::sleep(Duration::from_millis(100));
        let r2 = service.get_health();
        assert!(r2.uptime_seconds >= r1.uptime_seconds);
    }

    #[test]
    fn test_commit_from_env() {
        std::env::set_var("GIT_COMMIT", "abc123");
        let service = HealthService::new();
        let response = service.get_health();
        assert_eq!(response.commit, "abc123");
        std::env::remove_var("GIT_COMMIT");
    }

    #[test]
    fn test_commit_fallback_to_unknown() {
        std::env::remove_var("GIT_COMMIT");
        let service = HealthService::new();
        let response = service.get_health();
        assert_eq!(response.commit, "unknown");
    }
}
