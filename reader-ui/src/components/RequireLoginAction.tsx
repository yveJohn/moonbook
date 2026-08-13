import { useNavigate } from 'react-router-dom';
import { Button } from 'animal-island-ui';
import { useReaderAuth } from '../auth/ReaderAuthContext';

interface RequireLoginActionProps {
  children: React.ReactNode;
  redirectTo: string;
}

export function RequireLoginAction({ children, redirectTo }: RequireLoginActionProps) {
  const navigate = useNavigate();
  const { isAuthenticated } = useReaderAuth();

  if (isAuthenticated) {
    return <>{children}</>;
  }

  return (
    <Button type="primary" onClick={() => navigate(`/auth/login?redirect=${encodeURIComponent(redirectTo)}`)}>
      登录后继续
    </Button>
  );
}
