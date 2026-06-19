import React, { Component, ErrorInfo, ReactNode } from 'react';
import { trackError } from '../services/telemetry';

interface ErrorBoundaryProps {
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    this.setState({ errorInfo });

    // Log to console
    console.error('ErrorBoundary caught an error:', error, errorInfo);

    // Log to telemetry
    trackError(error, 'ErrorBoundary');

    // Call custom error handler
    this.props.onError?.(error, errorInfo);
  }

  private handleTryAgain = (): void => {
    this.setState({
      hasError: false,
      error: null,
      errorInfo: null,
    });
  };

  private handleCopyError = (): void => {
    const { error, errorInfo } = this.state;
    if (!error) return;

    const errorDetails = [
      `Error: ${error.message}`,
      '',
      'Stack Trace:',
      error.stack || 'No stack trace available',
      '',
      'Component Stack:',
      errorInfo?.componentStack || 'No component stack available',
    ].join('\n');

    navigator.clipboard.writeText(errorDetails).then(
      () => {
        console.log('Error details copied to clipboard');
      },
      (err) => {
        console.error('Failed to copy error details:', err);
      }
    );
  };

  render(): ReactNode {
    const { hasError, error } = this.state;
    const { fallback, children } = this.props;

    if (hasError) {
      // If a custom fallback is provided, use it
      if (fallback) {
        return fallback;
      }

      // Default fallback UI
      return (
        <div style={{
          padding: '20px',
          margin: '10px',
          border: '1px solid #ff6b6b',
          borderRadius: '8px',
          backgroundColor: '#fff5f5',
          fontFamily: 'system-ui, sans-serif',
        }}>
          <h3 style={{ color: '#d63031', margin: '0 0 10px 0' }}>
            Something went wrong
          </h3>
          <p style={{ color: '#636e72', margin: '0 0 15px 0' }}>
            {error?.message || 'An unexpected error occurred'}
          </p>
          <div style={{ display: 'flex', gap: '10px' }}>
            <button
              onClick={this.handleTryAgain}
              style={{
                padding: '8px 16px',
                backgroundColor: '#0984e3',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer',
                fontSize: '14px',
              }}
            >
              Try Again
            </button>
            <button
              onClick={this.handleCopyError}
              style={{
                padding: '8px 16px',
                backgroundColor: '#636e72',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer',
                fontSize: '14px',
              }}
            >
              Copy Error Details
            </button>
          </div>
        </div>
      );
    }

    return children;
  }
}
