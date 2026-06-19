use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use uuid::Uuid;

#[derive(Debug, Clone)]
pub struct RequestIdState {
    pub request_id: String,
    pub headers: HashMap<String, String>,
}

impl RequestIdState {
    pub fn new(request_id: String) -> Self {
        let mut headers = HashMap::new();
        headers.insert("X-Request-Id".into(), request_id.clone());
        Self { request_id, headers }
    }

    pub fn from_header(header_value: Option<&str>) -> Self {
        match header_value {
            Some(val) if !val.is_empty() && val.len() < 128 => {
                Self::new(val.to_string())
            }
            _ => Self::new(Uuid::new_v4().to_string()),
        }
    }
}

#[derive(Debug, Clone)]
pub struct RequestIdTracker {
    state: Arc<RwLock<HashMap<String, RequestIdState>>>,
}

impl RequestIdTracker {
    pub fn new() -> Self {
        Self {
            state: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub async fn generate_id(&self) -> String {
        Uuid::new_v4().to_string()
    }

    pub async fn track(&self, request_id: String) -> RequestIdState {
        let state = RequestIdState::new(request_id.clone());
        let mut map = self.state.write().await;
        map.insert(request_id, state.clone());
        state
    }

    pub async fn resolve(&self, header_value: Option<&str>) -> RequestIdState {
        let state = RequestIdState::from_header(header_value);
        let mut map = self.state.write().await;
        map.insert(state.request_id.clone(), state.clone());
        state
    }

    pub async fn cleanup(&self, request_id: &str) {
        let mut map = self.state.write().await;
        map.remove(request_id);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_request_id_state_creation() {
        let state = RequestIdState::new("test-123".into());
        assert_eq!(state.request_id, "test-123");
        assert_eq!(state.headers.get("X-Request-Id").unwrap(), "test-123");
    }

    #[test]
    fn test_request_id_from_header_valid() {
        let state = RequestIdState::from_header(Some("valid-id"));
        assert_eq!(state.request_id, "valid-id");
    }

    #[test]
    fn test_request_id_from_header_empty_generates_uuid() {
        let state = RequestIdState::from_header(Some(""));
        assert!(!state.request_id.is_empty());
        assert!(state.request_id.contains('-'));
    }

    #[test]
    fn test_request_id_from_header_too_long_generates_uuid() {
        let long_id = "a".repeat(128);
        let state = RequestIdState::from_header(Some(&long_id));
        assert!(!state.request_id.is_empty());
        assert_ne!(state.request_id, long_id);
    }

    #[test]
    fn test_request_id_from_header_none_generates_uuid() {
        let state = RequestIdState::from_header(None);
        assert!(!state.request_id.is_empty());
        assert!(state.request_id.contains('-'));
    }

    #[tokio::test]
    async fn test_tracker_generate_id() {
        let tracker = RequestIdTracker::new();
        let id = tracker.generate_id().await;
        assert!(!id.is_empty());
        assert!(id.contains('-'));
    }

    #[tokio::test]
    async fn test_tracker_resolve_with_header() {
        let tracker = RequestIdTracker::new();
        let state = tracker.resolve(Some("my-request-id")).await;
        assert_eq!(state.request_id, "my-request-id");
    }

    #[tokio::test]
    async fn test_tracker_resolve_without_header() {
        let tracker = RequestIdTracker::new();
        let state = tracker.resolve(None).await;
        assert!(!state.request_id.is_empty());
    }
}
