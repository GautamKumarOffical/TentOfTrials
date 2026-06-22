use anyhow::{bail, Result};
use std::env;
use tracing_subscriber::EnvFilter;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogFormat {
    Text,
    Json,
}

impl LogFormat {
    pub fn as_str(&self) -> &'static str {
        match self {
            LogFormat::Text => "text",
            LogFormat::Json => "json",
        }
    }
}

pub fn parse_log_format() -> Result<LogFormat> {
    let value = env::var("TOT_LOG_FORMAT").unwrap_or_default();
    match value.as_str() {
        "" | "text" => Ok(LogFormat::Text),
        "json" => Ok(LogFormat::Json),
        other => bail!("invalid TOT_LOG_FORMAT value: {other:?} (expected \"text\" or \"json\")"),
    }
}

pub fn init_logging() -> Result<LogFormat> {
    let format = parse_log_format()?;

    let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into());

    let subscriber = tracing_subscriber::fmt().with_env_filter(filter);

    match format {
        LogFormat::Text => subscriber.init(),
        LogFormat::Json => subscriber.json().init(),
    }

    tracing::info!(
        backend_log_format = format.as_str(),
        "logging initialized"
    );

    Ok(format)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn default_is_text() {
        env::remove_var("TOT_LOG_FORMAT");
        let fmt = parse_log_format().unwrap();
        assert_eq!(fmt, LogFormat::Text);
        assert_eq!(fmt.as_str(), "text");
    }

    #[test]
    fn explicit_text() {
        env::set_var("TOT_LOG_FORMAT", "text");
        let fmt = parse_log_format().unwrap();
        assert_eq!(fmt, LogFormat::Text);
    }

    #[test]
    fn json_format() {
        env::set_var("TOT_LOG_FORMAT", "json");
        let fmt = parse_log_format().unwrap();
        assert_eq!(fmt, LogFormat::Json);
        assert_eq!(fmt.as_str(), "json");
    }

    #[test]
    fn invalid_value_fails() {
        env::set_var("TOT_LOG_FORMAT", "xml");
        let result = parse_log_format();
        assert!(result.is_err());
        let err = result.unwrap_err().to_string();
        assert!(err.contains("xml"), "error should mention the bad value: {err}");
    }
}
