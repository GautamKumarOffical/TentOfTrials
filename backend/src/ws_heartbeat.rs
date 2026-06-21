use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};
use std::env;:

const DEFAULT_HEARTBEAT_INTERVAL_SECS: u64 = 30;

#[derive(Debug, Clone)]
pub struct ConnectionState {
    pub last_pong: Instant,
    pub connected_at: Instant,
}

impl Default for ConnectionState {
    fn default() -> Self {
        Self {
            last_pong: Instant::now(),
            connected_at: Instant::now(),
        }
    }
}

#[derive(Debug, Clone)]
pub struct WsHeartbeatManager {
    connections: Arc<RwLock<HashMap<String, ConnectionState>>>,
    heartbeat_interval: Duration,
}

impl WsHeartbeatManager {
    pub fn new() -> Self {
        let interval_secs = env::var("WS_HEARTBEAT_INTERVAL_SECS")
            .ok()
            .and_then(|s| s.parse().ok())
            .unwrap_or(DEFAULT_HEARTBEAT_INTERVAL_SECS);
        
        Self {
            connections: Arc::new(RwLock::new(HashMap::new())),
            heartbeat_interval: Duration::from_secs(interval_secs),
        }
    }

    pub fn with_interval(secs: u64) -> Self {
        Self {
            connections: Arc::new(RwLock::new(HashMap::new())),
            heartbeat_interval: Duration::from_secs(secs),
        }
    }

    pub fn register_connection(&self, conn_id: String) {
        let mut conns = self.connections.write().unwrap();
        conns.insert(conn_id, ConnectionState::default());
    }

    pub fn unregister_connection(&self, conn_id: &str) {
        let mut conns = self.connections.write().unwrap();
        conns.remove(conn_id);
    }

    pub fn record_pong(&self, conn_id: &str) {
        let mut conns = self.connections.write().unwrap();
        if let Some(state) = conns.get_mut(conn_id) {
            state.last_pong = Instant::now();
        }
    }

    pub fn get_idle_connections(&self) -> Vec<String> {
        let conns = self.connections.read().unwrap();
        let timeout = self.heartbeat_interval * 2;
        let now = Instant::now();
        
        conns.iter()
            .filter(|(_, state)| now.duration_since(state.last_pong) > timeout)
            .map(|(id, _)| id.clone())
            .collect()
    }

    pub fn active_connection_count(&self) -> usize {
        self.connections.read().unwrap().len()
    }

    pub fn heartbeat_interval_secs(&self) -> u64 {
        self.heartbeat_interval.as_secs()
    }

    pub fn health_status(&self) -> HealthStatus {
        HealthStatus {
            active_ws: self.active_connection_count(),
            heartbeat_secs: self.heartbeat_interval_secs(),
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct HealthStatus {
    pub active_ws: usize,
    pub heartbeat_secs: u64,
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread;

    #[test]
    fn test_default_heartbeat_interval() {
        let manager = WsHeartbeatManager::with_interval(30);
        assert_eq!(manager.heartbeat_interval_secs(), 30);
    }

    #[test]
    fn test_register_connection() {
        let manager = WsHeartbeatManager::with_interval(30);
        assert_eq!(manager.active_connection_count(), 0);
        
        manager.register_connection("conn1".to_string());
        assert_eq!(manager.active_connection_count(), 1);
        
        manager.register_connection("conn2".to_string());
        assert_eq!(manager.active_connection_count(), 2);
    }

    #[test]
    fn test_unregister_connection() {
        let manager = WsHeartbeatManager::with_interval(30);
        manager.register_connection("conn1".to_string());
        assert_eq!(manager.active_connection_count(), 1);
        
        manager.unregister_connection("conn1");
        assert_eq!(manager.active_connection_count(), 0);
    }

    #[test]
    fn test_record_pong() {
        let manager = WsHeartbeatManager::with_interval(30);
        manager.register_connection("conn1".to_string());
        
        // Should not panic
        manager.record_pong("conn1");
        manager.record_pong("nonexistent");
    }

    #[test]
    fn test_idle_connections_none_initially() {
        let manager = WsHeartbeatManager::with_interval(30);
        manager.register_connection("conn1".to_string());
        
        let idle = manager.get_idle_connections();
        assert!(idle.is_empty());
    }

    #[test]
    fn test_idle_connections_after_timeout() {
        // Use 1ms interval for fast testing
        let manager = WsHeartbeatManager {
            connections: Arc::new(RwLock::new(HashMap::new())),
            heartbeat_interval: Duration::from_millis(1),
        };
        manager.register_connection("conn1".to_string());
        
        // Wait for 2x timeout
        thread::sleep(Duration::from_millis(5));
        
        let idle = manager.get_idle_connections();
        assert_eq!(idle.len(), 1);
        assert_eq!(idle[0], "conn1");
    }

    #[test]
    fn test_pong_resets_idle_timer() {
        let manager = WsHeartbeatManager {
            connections: Arc::new(RwLock::new(HashMap::new())),
            heartbeat_interval: Duration::from_millis(10),
        };
        manager.register_connection("conn1".to_string());
        
        thread::sleep(Duration::from_millis(15));
        manager.record_pong("conn1");
        
        let idle = manager.get_idle_connections();
        assert!(idle.is_empty());
    }

    #[test]
    fn test_health_status() {
        let manager = WsHeartbeatManager::with_interval(45);
        manager.register_connection("conn1".to_string());
        manager.register_connection("conn2".to_string());
        
        let status = manager.health_status();
        assert_eq!(status.active_ws, 2);
        assert_eq!(status.heartbeat_secs, 45);
    }

    #[test]
    fn test_custom_interval() {
        let manager = WsHeartbeatManager::with_interval(60);
        assert_eq!(manager.heartbeat_interval_secs(), 60);
    }
}