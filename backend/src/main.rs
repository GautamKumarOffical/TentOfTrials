use anyhow::Result;
use clap::Parser;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use tent_backend::discovery::ServiceDiscovery;
use tent_backend::messaging::MessageBroker;
use tent_backend::registry::ServiceRegistry;
use tokio::time::{sleep, Duration};
use tracing_subscriber::EnvFilter;

#[derive(Parser, Debug)]
#[command(name = "tent-backend")]
#[command(about = "Tent of Trials Backend - Distributed Microservices Framework", long_about = None)]
struct Cli {
    #[arg(short, long, default_value = "node-0")]
    node_id: String,

    #[arg(short, long)]
    consensus: bool,

    #[arg(long, default_value_t = 10000)]
    max_connections: u32,

    #[arg(short, long, default_value = "/etc/tent/config.toml")]
    config: String,
}

pub struct GracefulShutdown {
    shutdown_initiated: Arc<AtomicBool>,
    in_flight_requests: Arc<AtomicU64>,
    grace_secs: u64,
}

impl GracefulShutdown {
    pub fn new(grace_secs: u64) -> Self {
        Self {
            shutdown_initiated: Arc::new(AtomicBool::new(false)),
            in_flight_requests: Arc::new(AtomicU64::new(0)),
            grace_secs,
        }
    }

    pub fn begin(&self) {
        self.shutdown_initiated.store(true, Ordering::SeqCst);
        tracing::info!(
            grace_period_secs = self.grace_secs,
            in_flight = self.in_flight_requests.load(Ordering::SeqCst),
            "graceful shutdown initiated, stopping new request acceptance"
        );
    }

    pub fn is_shutting_down(&self) -> bool {
        self.shutdown_initiated.load(Ordering::SeqCst)
    }

    pub fn request_started(&self) {
        self.in_flight_requests.fetch_add(1, Ordering::SeqCst);
    }

    pub fn request_finished(&self) {
        self.in_flight_requests.fetch_sub(1, Ordering::SeqCst);
    }

    pub async fn wait_for_drain(&self) {
        let deadline = Duration::from_secs(self.grace_secs);
        let start = tokio::time::Instant::now();

        loop {
            let remaining = self.in_flight_requests.load(Ordering::SeqCst);
            if remaining == 0 {
                tracing::info!("all in-flight requests completed");
                return;
            }

            if start.elapsed() >= deadline {
                tracing::warn!(
                    in_flight = remaining,
                    grace_period_secs = self.grace_secs,
                    "grace period expired with in-flight requests still pending"
                );
                return;
            }

            tracing::debug!(
                in_flight = remaining,
                elapsed_ms = start.elapsed().as_millis() as u64,
                "waiting for in-flight requests to complete"
            );
            sleep(Duration::from_millis(100)).await;
        }
    }
}

fn parse_grace_period() -> u64 {
    std::env::var("TOT_SHUTDOWN_GRACE_SECS")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(5)
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()))
        .json()
        .init();

    let cli = Cli::parse();
    let grace_secs = parse_grace_period();
    let shutdown = Arc::new(GracefulShutdown::new(grace_secs));

    tracing::info!(
        node_id = %cli.node_id,
        consensus = %cli.consensus,
        max_connections = %cli.max_connections,
        config = %cli.config,
        shutdown_grace_secs = %grace_secs,
        "initializing tent backend orchestration framework"
    );

    let config = tent_backend::config::load_config(&cli.config).await?;
    let registry = ServiceRegistry::new(config.registry.clone());
    let discovery = ServiceDiscovery::new(config.discovery.clone());
    let broker = MessageBroker::new(config.messaging.clone());

    registry.initialize().await?;
    discovery.announce(&cli.node_id).await?;
    broker.connect().await?;

    tracing::info!("all subsystems initialized successfully, entering main loop");

    let mut signal = tokio::signal::unix::signal(
        tokio::signal::unix::SignalKind::terminate(),
    )?;

    tokio::select! {
        _ = signal.recv() => {
            tracing::info!("received SIGTERM, initiating graceful shutdown");
        }
        _ = tokio::signal::ctrl_c() => {
            tracing::info!("received SIGINT, initiating graceful shutdown");
        }
    }

    shutdown.begin();

    shutdown.wait_for_drain().await;

    tracing::info!("draining complete, tearing down subsystems");

    broker.disconnect().await?;
    discovery.withdraw(&cli.node_id).await?;
    registry.shutdown().await?;

    tracing::info!("shutdown complete");
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    #[test]
    fn grace_period_default_is_five_seconds() {
        let val = parse_grace_period();
        assert_eq!(val, 5);
    }

    #[test]
    fn grace_period_respects_env_var() {
        std::env::set_var("TOT_SHUTDOWN_GRACE_SECS", "30");
        let val = parse_grace_period();
        assert_eq!(val, 30);
        std::env::remove_var("TOT_SHUTDOWN_GRACE_SECS");
    }

    #[test]
    fn grace_period_falls_back_on_invalid_env() {
        std::env::set_var("TOT_SHUTDOWN_GRACE_SECS", "not_a_number");
        let val = parse_grace_period();
        assert_eq!(val, 5);
        std::env::remove_var("TOT_SHUTDOWN_GRACE_SECS");
    }

    #[test]
    fn shutdown_flag_toggles_correctly() {
        let s = GracefulShutdown::new(5);
        assert!(!s.is_shutting_down());
        s.begin();
        assert!(s.is_shutting_down());
    }

    #[test]
    fn in_flight_counter_tracks_started_and_finished() {
        let s = GracefulShutdown::new(5);
        assert_eq!(s.in_flight_requests.load(Ordering::SeqCst), 0);

        s.request_started();
        s.request_started();
        assert_eq!(s.in_flight_requests.load(Ordering::SeqCst), 2);

        s.request_finished();
        assert_eq!(s.in_flight_requests.load(Ordering::SeqCst), 1);

        s.request_finished();
        assert_eq!(s.in_flight_requests.load(Ordering::SeqCst), 0);
    }

    #[tokio::test]
    async fn drain_returns_immediately_when_no_in_flight() {
        let s = GracefulShutdown::new(1);
        s.begin();
        s.wait_for_drain().await;
    }

    #[tokio::test]
    async fn drain_completes_after_requests_finish() {
        let s = Arc::new(GracefulShutdown::new(10));
        s.request_started();

        let s2 = s.clone();
        tokio::spawn(async move {
            sleep(Duration::from_millis(200)).await;
            s2.request_finished();
        });

        s.begin();
        s.wait_for_drain().await;
    }

    #[tokio::test]
    async fn drain_expires_when_grace_period_runs_out() {
        let s = Arc::new(GracefulShutdown::new(1));
        s.request_started();

        let s2 = s.clone();
        tokio::spawn(async move {
            sleep(Duration::from_secs(5)).await;
            s2.request_finished();
        });

        s.begin();
        let start = tokio::time::Instant::now();
        s.wait_for_drain().await;
        let elapsed = start.elapsed();

        assert!(elapsed >= Duration::from_secs(1));
        assert!(elapsed < Duration::from_secs(3));
    }
}
