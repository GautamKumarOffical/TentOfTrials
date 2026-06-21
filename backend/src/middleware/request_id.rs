//! Request ID propagation middleware
//! 
//! Extracts or generates a unique request ID for each incoming request,
//! propagates it through the request context, and includes it in the response.

use std::task::{Context, Poll};
use std::future::Future;
use std::pin::Pin;
use uuid::Uuid;

/// The header name for request ID propagation
pub const REQUEST_ID_HEADER: &str = "X-RK	uKt-ID";

/// Request ID that can be extracted from request extensions
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct RequestId(pub String);

impl RequestId {
    /// Create a new random request ID
    pub fn new() -> Self {
        Self(Uuid::new_v4().to_string())
    }

    /// Create a request ID from an existing string
    pub fn from_string(id: String) -> Self {
        Self(id)
    }

    /// Get the request ID as a string slice
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl std::fmt::Display for RequestId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Default for RequestId {
    fn default() -> Self {
        Self::new()
    }
}

/// Middleware layer for request ID propagation
#[derive(Clone, Debug)]
pub struct RequestIdLayer;

impl RequestIdLayer {
    /// Create a new request ID layer
    pub fn new() -> Self {
        Self
    }
}

impl Default for RequestIdLayer {
    fn default() -> Self {
        Self::new()
    }
}

impl<S> tower::Layer<S> for RequestIdLayer {
    type Service = RequestIdService<S>;

    fn layer(&self, inner: S) -> Self::Service {
        RequestIdService { inner }
    }
}

/// Middleware service that propagates request IDs
#[derive(Clone, Debug)]
pub struct RequestIdService<S> {
    inner: S,
}

impl<S, B> tower::Service<http::Request<B>> for RequestIdService<S>
where
    S: tower::Service<http::Request<B>> + Clone + Send + 'static,
    S::Response: Into<http::Response<http::Response<()>>>,
    S::Future: Send,
    B: Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = S::Future;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, mut req: http::Request<B>) -> Self::Future {
        // Extract existing request ID or generate a new one
        let request_id = req
            .headers()
            .get(REQUEST_ID_HEADER)
            .and_then(|v| v.to_str().ok())
            .map(|s| RequestId::from_string(s.to_string()))
            .unwrap_or_else(RequestId::new);

        // Insert request ID into request extensions for downstream access
        req.extensions_mut().insert(request_id);

        self.inner.call(req)
    }
}

/// Extract request ID from headers or generate a new one
pub fn extract_or_generate_request_id(headers: &http::HeaderMap) -> RequestId {
    headers
        .get(REQUEST_ID_HEADER)
        .and_then(|v| v.to_str().ok())
        .map(|s| RequestId::from_string(s.to_string()))
        .unwrap_or_else(RequestId::new)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_request_id_new() {
        let id1 = RequestId::new();
        let id2 = RequestId::new();
        assert_ne!(id1, id2, "Each new request ID should be unique");
    }

    #[test]
    fn test_request_id_from_string() {
        let id = RequestId::from_string("test-id-123".to_string());
        assert_eq!(id.as_str(), "test-id-123");
    }

    #[test]
    fn test_request_id_display() {
        let id = RequestId::from_string("display-test".to_string());
        assert_eq!(format!("{}", id), "display-test");
    }

    #[test]
    fn test_extract_or_generate_with_existing_header() {
        let mut headers = http::HeaderMap::new();
        headers.insert(
            http::header::HeaderName::from_static("x-request-id"),
            http::header::HeaderValue::from_static("existing-id-456"),
        );
        let id = extract_or_generate_request_id(&headers);
        assert_eq!(id.as_str(), "existing-id-456");
    }

    #[test]
    fn test_extract_or_generate_without_header() {
        let headers = http::HeaderMap::new();
        let id = extract_or_generate_request_id(&headers);
        assert!(!id.as_str().is_empty(), "Should generate a new ID");
    }
}
