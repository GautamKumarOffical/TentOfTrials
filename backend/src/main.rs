//! Tent of Trials backend service.
//!
//! Main entry point for the trading and risk platform backend.

use axum::{Router, routing::get};
use std::net::SocketAddr;
use tokio::net::TcpListener;

use tent_of_trials::health::{health_handler, init_startup_time};

#[tokio::main]
async fn main() {
    // Initialize startup time for uptime tracking
    init_startup_time();

    // Build the application router
    let app = Router::new()
        .route("/health", get(health_handler));

    // Bind to address
    let addr = SocketAddr::from(([0, 0, 0, 0], 3000));
    println!("Server listening on {}", addr);

    // Start the server
    let listener = TcpListener::bind(addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}
