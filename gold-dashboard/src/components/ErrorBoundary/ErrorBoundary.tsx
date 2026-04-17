import { Component, type ErrorInfo, type ReactNode } from 'react';
import './ErrorBoundary.css';

interface Props {
  children: ReactNode;
  fallback?: (error: Error, reset: () => void) => ReactNode;
  onReset?: () => void;
  resetKeys?: unknown[];
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('[ErrorBoundary]', error, info.componentStack);
  }

  componentDidUpdate(prevProps: Props): void {
    const { resetKeys } = this.props;
    if (this.state.error && resetKeys && prevProps.resetKeys !== resetKeys) {
      const changed = resetKeys.some((key, i) => key !== (prevProps.resetKeys?.[i]));
      if (changed) this.reset();
    }
  }

  reset = (): void => {
    this.props.onReset?.();
    this.setState({ error: null });
  };

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    if (this.props.fallback) return this.props.fallback(error, this.reset);

    return (
      <div className="error-boundary" role="alert">
        <p className="error-boundary__message">{error.message || 'Something went wrong'}</p>
        <button type="button" className="error-boundary__retry" onClick={this.reset}>
          Retry
        </button>
      </div>
    );
  }
}
