//! Configuration loader module
//! 
//! Provides multi-format configuration loading with environment variable overrides.

use serde::{Deserialize, Serialize};
use std::env;
use std::fs;
use std::path::Path;
use thiserror::Error;

/// Configuration error types
#[derive(Error, Debug)]
pub enum ConfigError {
    #[error("Failed to read config file: {0}")]
    FileRead(#[from] std::io::Error),
    
    #[error("Failed to parse TOML: {0}")]
    TomlParse(#[from] toml::de::Error),
    
    #[error("Failed to parse JSON: {0}")]
    JsonParse(#[from] serde_json::Error),
    
    #[error("Failed to parse YAML: {0}")]
    YamlParse(#[from] serde_yaml::Error),
    
    #[error("Unsupported config format: {0}")]
    UnsupportedFormat(String),
    
    #[error("Validation error: {0}")]
    Validation(String),
}

/// Application configuration structure
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AppConfig {
    #[serde(default)]
    pub server: ServerConfig,
    #[serde(default)]
    pub database: DatabaseConfig,
    #[serde(default)]
    pub ai: AiConfig,
}

/// Server configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    #[serde(default = "default_host")]
    pub host: String,
    #[serde(default = "default_port")]
    pub port: u16,
}

fn default_host() -> String { "127.0.0.1".to_string() }
fn default_port() -> u16 { 8080 }

impl Default for ServerConfig {
    fn default() -> Self {
        Self { host: default_host(), port: default_port() }
    }
}

/// Database configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DatabaseConfig {
    #[serde(default = "default_db_url")]
    pub url: String,
    #[serde(default = "default_pool_size")]
    pub pool_size: u32,
}

fn default_db_url() -> String { "sqlite://:memory:".to_string() }
fn default_pool_size() -> u32 { 5 }

impl Default for DatabaseConfig {
    fn default() -> Self {
        Self { url: default_db_url(), pool_size: default_pool_size() }
    }
}

/// AI configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AiConfig {
    #[serde(default)]
    pub model_path: Option<String>,
    #[serde(default = "default_max_tokens")]
    pub max_tokens: usize,
}

fn default_max_tokens() -> usize { 2048 }

impl Default for AiConfig {
    fn default() -> Self {
        Self { model_path: None, max_tokens: default_max_tokens() }
    }
}

/// Configuration loader with multi-format support
pub struct ConfigLoader;

impl ConfigLoader {
    /// Load configuration from a file with environment variable overrides
    pub fn load<P: AsRef<Path>>(path: P) -> Result<AppConfig, ConfigError> {
        let path = path.as_ref();
        let mut config = if path.exists() {
            Self::load_from_file(path)?
        } else {
            AppConfig::default()
        };
        Self::apply_env_overrides(&mut config);
        Self::validate(&config)?;
        Ok(config)
    }

    /// Load configuration from a file based on extension
    fn load_from_file(path: &Path) -> Result<AppConfig, ConfigError> {
        let content = fs::read_to_string(path)?;
        let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");
        
        match ext.to_lowercase().as_str() {
            "toml" => Ok(toml::from_str(&content)?),
            "json" => Ok(serde_json::from_str(&content)?),
            "yaml" | "yml" => Ok(serde_yaml::from_str(&content)?),
            _ => Err(ConfigError::UnsupportedFormat(ext.to_string())),
        }
    }

    /// Apply environment variable overrides
    fn apply_env_overrides(config: &mut AppConfig) {
        if let Ok(host) = env::var("APP_SERVER_HOST") {
            config.server.host = host;
        }
        if let Ok(port) = env::var("APP_SERVER_PORT") {
            if let Ok(p) = port.parse() {
                config.server.port = p;
            }
        }
        if let Ok(url) = env::var("APP_DATABASE_URL") {
            config.database.url = url;
        }
    }

    /// Validate configuration
    fn validate(config: &AppConfig) -> Result<(), ConfigError> {
        if config.server.port == 0 {
            return Err(ConfigError::Validation("Port cannot be 0".to_string()));
        }
        if config.database.pool_size == 0 {
            return Err(ConfigError::Validation("Pool size cannot be 0".to_string()));
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_default_config() {
        let config = AppConfig::default();
        assert_eq!(config.server.host, "127.0.0.1");
        assert_eq!(config.server.port, 8080);
    }

    #[test]
    fn test_load_toml() {
        let mut file = NamedTempFile::with_suffix(".toml").unwrap();
        writeln!(file, "[server]\nhost = \"0.0.0.0\"\nport = 3000").unwrap();
        
        let config = ConfigLoader::load(file.path()).unwrap();
        assert_eq!(config.server.host, "0.0.0.0");
        assert_eq!(config.server.port, 3000);
    }

    #[test]
    fn test_load_json() {
        let mut file = NamedTempFile::with_suffix(".json").unwrap();
        writeln!(file, r{{"server": {{"host": "localhost", "port": 9000}}}}).unwrap();
        
        let config = ConfigLoader::load(file.path()).unwrap();
        assert_eq!(config.server.host, "localhost");
        assert_eq!(config.server.port, 9000);
    }

    #[test]
    fn test_validation_fails() {
        let mut file = NamedTempFile::with_suffix(".toml").unwrap();
        writeln!(file, "[server]\nport = 0").unwrap();
        
        let result = ConfigLoader::load(file.path());
        assert!(result.is_err());
    }
}