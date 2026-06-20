use std::env;
use std::fmt;
use thiserror::Error;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogFormat {
    Text,
    Json,
}

#[derive(Debug, Error)]
pub enum LogFormatError {
    #[error("invalid TOT_LOG_FORMAT value '{0}': expected 'text' or 'json'")]
    InvalidFormat(String),
}

impl LogFormat {
    pub fn from_env() -> Result<Self, LogFormatError> {
        match env::var("TOT_LOG_FORMAT") {
            Ok(val) => val.parse(),
            Err(env::VarError::NotPresent) => Ok(LogFormat::Text),
            Err(e) => Err(LogFormatError::InvalidFormat(e.to_string())),
        }
    }
}

impl fmt::Display for LogFormat {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            LogFormat::Text => write!(f, "text"),
            LogFormat::Json => write!(f, "json"),
        }
    }
}

impl std::str::FromStr for LogFormat {
    type Err = LogFormatError;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "text" => Ok(LogFormat::Text),
            "json" => Ok(LogFormat::Json),
            _ => Err(LogFormatError::InvalidFormat(s.to_string())),
        }
    }
}

pub fn init_logging() -> Result<LogFormat, LogFormatError> {
    let format = LogFormat::from_env()?;

    let subscriber = tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info".into()),
        );

    match format {
        LogFormat::Json => {
            subscriber.json().init();
        }
        LogFormat::Text => {
            subscriber.init();
        }
    }

    Ok(format)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn test_log_format_default_is_text() {
        env::remove_var("TOT_LOG_FORMAT");
        assert_eq!(LogFormat::from_env().unwrap(), LogFormat::Text);
    }

    #[test]
    fn test_log_format_json_from_env() {
        env::set_var("TOT_LOG_FORMAT", "json");
        assert_eq!(LogFormat::from_env().unwrap(), LogFormat::Json);
        env::remove_var("TOT_LOG_FORMAT");
    }

    #[test]
    fn test_log_format_text_from_env() {
        env::set_var("TOT_LOG_FORMAT", "text");
        assert_eq!(LogFormat::from_env().unwrap(), LogFormat::Text);
        env::remove_var("TOT_LOG_FORMAT");
    }

    #[test]
    fn test_log_format_case_insensitive() {
        env::set_var("TOT_LOG_FORMAT", "JSON");
        assert_eq!(LogFormat::from_env().unwrap(), LogFormat::Json);
        env::set_var("TOT_LOG_FORMAT", "Text");
        assert_eq!(LogFormat::from_env().unwrap(), LogFormat::Text);
        env::set_var("TOT_LOG_FORMAT", "Json");
        assert_eq!(LogFormat::from_env().unwrap(), LogFormat::Json);
        env::remove_var("TOT_LOG_FORMAT");
    }

    #[test]
    fn test_log_format_invalid_value() {
        env::set_var("TOT_LOG_FORMAT", "xml");
        let err = LogFormat::from_env().unwrap_err();
        assert!(matches!(err, LogFormatError::InvalidFormat(_)));
        assert!(err.to_string().contains("xml"));
        env::remove_var("TOT_LOG_FORMAT");
    }

    #[test]
    fn test_log_format_parse_trait() {
        assert_eq!("json".parse::<LogFormat>().unwrap(), LogFormat::Json);
        assert_eq!("text".parse::<LogFormat>().unwrap(), LogFormat::Text);
        assert!("yaml".parse::<LogFormat>().is_err());
    }

    #[test]
    fn test_log_format_display() {
        assert_eq!(LogFormat::Text.to_string(), "text");
        assert_eq!(LogFormat::Json.to_string(), "json");
    }
}
