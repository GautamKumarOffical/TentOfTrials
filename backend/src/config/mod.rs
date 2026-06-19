use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceConfig {
    pub name: String,
    pub version: String,
    pub host: String,
    pub port: u16,
    pub tls_enabled: bool,
    pub tls_cert_path: Option<String>,
    pub tls_key_path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RegistryConfig {
    pub backend: String,
    pub endpoints: Vec<String>,
    pub heartbeat_interval_ms: u64,
    pub ttl_seconds: u64,
    pub replication_factor: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiscoveryConfig {
    pub provider: String,
    pub namespace: String,
    pub tags: Vec<String>,
    pub health_check_path: String,
    pub health_check_interval_ms: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MessagingConfig {
    pub broker_type: String,
    pub uris: Vec<String>,
    pub consumer_group: String,
    pub max_retries: u32,
    pub retry_backoff_ms: u64,
    pub batch_size: u32,
    pub compression: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RootConfig {
    pub service: ServiceConfig,
    pub registry: RegistryConfig,
    pub discovery: DiscoveryConfig,
    pub messaging: MessagingConfig,
}

impl Default for RootConfig {
    fn default() -> Self {
        Self {
            service: ServiceConfig {
                name: "tent-backend".into(),
                version: "0.1.0".into(),
                host: "0.0.0.0".into(),
                port: 8080,
                tls_enabled: false,
                tls_cert_path: None,
                tls_key_path: None,
            },
            registry: RegistryConfig {
                backend: "etcd".into(),
                endpoints: vec!["localhost:2379".into()],
                heartbeat_interval_ms: 5000,
                ttl_seconds: 30,
                replication_factor: 3,
            },
            discovery: DiscoveryConfig {
                provider: "consul".into(),
                namespace: "tent".into(),
                tags: vec!["microservice".into(), "orchestration".into()],
                health_check_path: "/health".into(),
                health_check_interval_ms: 10000,
            },
            messaging: MessagingConfig {
                broker_type: "kafka".into(),
                uris: vec!["localhost:9092".into()],
                consumer_group: "tent-consumers".into(),
                max_retries: 3,
                retry_backoff_ms: 1000,
                batch_size: 500,
                compression: "snappy".into(),
            },
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EnvConfig {
    pub host: String,
    pub port: u16,
    pub log_level: String,
    pub enable_experimental: bool,
}

impl EnvConfig {
    pub fn from_env() -> Result<Self> {
        let host = std::env::var("TOT_BACKEND_HOST")
            .unwrap_or_else(|_| "0.0.0.0".into());

        let port = std::env::var("TOT_BACKEND_PORT")
            .unwrap_or_else(|_| "8080".into())
            .parse::<u16>()
            .context("Invalid TOT_BACKEND_PORT: must be a valid port number (1-65535)")?;

        let log_level = std::env::var("TOT_LOG_LEVEL")
            .unwrap_or_else(|_| "info".into());

        let enable_experimental = std::env::var("TOT_ENABLE_EXPERIMENTAL")
            .map(|v| match v.to_lowercase().as_str() {
                "true" | "1" | "yes" => Ok(true),
                "false" | "0" | "no" => Ok(false),
                _ => Err(anyhow::anyhow!(
                    "Invalid TOT_ENABLE_EXPERIMENTAL value '{}': must be true/false, 1/0, or yes/no",
                    v
                )),
            })
            .unwrap_or(Ok(false))?;

        Ok(Self {
            host,
            port,
            log_level,
            enable_experimental,
        })
    }
}

pub async fn load_config(path: &str) -> Result<RootConfig> {
    let path = Path::new(path);
    if path.exists() {
        let contents = tokio::fs::read_to_string(path).await?;
        let config: RootConfig = toml::from_str(&contents)?;
        tracing::info!("configuration loaded from {}", path.display());
        Ok(config)
    } else {
        tracing::warn!(
            "config file {} not found, using defaults",
            path.display()
        );
        Ok(RootConfig::default())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn test_env_config_defaults() {
        env::remove_var("TOT_BACKEND_HOST");
        env::remove_var("TOT_BACKEND_PORT");
        env::remove_var("TOT_LOG_LEVEL");
        env::remove_var("TOT_ENABLE_EXPERIMENTAL");

        let config = EnvConfig::from_env().unwrap();
        assert_eq!(config.host, "0.0.0.0");
        assert_eq!(config.port, 8080);
        assert_eq!(config.log_level, "info");
        assert!(!config.enable_experimental);
    }

    #[test]
    fn test_env_config_valid_overrides() {
        env::set_var("TOT_BACKEND_HOST", "127.0.0.1");
        env::set_var("TOT_BACKEND_PORT", "9090");
        env::set_var("TOT_LOG_LEVEL", "debug");
        env::set_var("TOT_ENABLE_EXPERIMENTAL", "true");

        let config = EnvConfig::from_env().unwrap();
        assert_eq!(config.host, "127.0.0.1");
        assert_eq!(config.port, 9090);
        assert_eq!(config.log_level, "debug");
        assert!(config.enable_experimental);

        env::remove_var("TOT_BACKEND_HOST");
        env::remove_var("TOT_BACKEND_PORT");
        env::remove_var("TOT_LOG_LEVEL");
        env::remove_var("TOT_ENABLE_EXPERIMENTAL");
    }

    #[test]
    fn test_env_config_invalid_port() {
        env::set_var("TOT_BACKEND_PORT", "invalid");
        let result = EnvConfig::from_env();
        assert!(result.is_err());
        env::remove_var("TOT_BACKEND_PORT");
    }

    #[test]
    fn test_env_config_port_out_of_range() {
        env::set_var("TOT_BACKEND_PORT", "99999");
        let result = EnvConfig::from_env();
        assert!(result.is_err());
        env::remove_var("TOT_BACKEND_PORT");
    }

    #[test]
    fn test_env_config_invalid_boolean() {
        env::set_var("TOT_ENABLE_EXPERIMENTAL", "maybe");
        let result = EnvConfig::from_env();
        assert!(result.is_err());
        env::remove_var("TOT_ENABLE_EXPERIMENTAL");
    }

    #[test]
    fn test_env_config_boolean_variants() {
        env::set_var("TOT_ENABLE_EXPERIMENTAL", "1");
        assert!(EnvConfig::from_env().unwrap().enable_experimental);
        env::set_var("TOT_ENABLE_EXPERIMENTAL", "yes");
        assert!(EnvConfig::from_env().unwrap().enable_experimental);
        env::set_var("TOT_ENABLE_EXPERIMENTAL", "false");
        assert!(!EnvConfig::from_env().unwrap().enable_experimental);
        env::set_var("TOT_ENABLE_EXPERIMENTAL", "0");
        assert!(!EnvConfig::from_env().unwrap().enable_experimental);
        env::remove_var("TOT_ENABLE_EXPERIMENTAL");
    }
}
