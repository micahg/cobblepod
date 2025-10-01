import { useAuth0 } from '@auth0/auth0-react';

const UserProfile = () => {
  const { user, isAuthenticated, isLoading } = useAuth0();

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (!isAuthenticated || !user) {
    return null;
  }

  return (
    <div
      className="user-profile"
      style={{
        padding: '16px',
        backgroundColor: '#f8f9fa',
        border: '1px solid #dee2e6',
        borderRadius: '8px',
        margin: '16px 0',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        {user.picture && (
          <img
            src={user.picture}
            alt={user.name || 'User'}
            style={{
              width: '48px',
              height: '48px',
              borderRadius: '50%',
            }}
          />
        )}
        <div>
          <h3 style={{ margin: '0 0 4px 0', fontSize: '18px' }}>
            {user.name || 'Anonymous User'}
          </h3>
          <p style={{ margin: '0', fontSize: '14px', color: '#6c757d' }}>
            {user.email}
          </p>
        </div>
      </div>
      
      {/* Debug info - you can remove this in production */}
      <details style={{ marginTop: '12px' }}>
        <summary style={{ cursor: 'pointer', fontSize: '12px', color: '#6c757d' }}>
          Debug Info
        </summary>
        <pre style={{ 
          fontSize: '10px', 
          backgroundColor: '#f1f3f4', 
          padding: '8px', 
          borderRadius: '4px',
          marginTop: '8px',
          overflow: 'auto',
          maxHeight: '200px'
        }}>
          {JSON.stringify(user, null, 2)}
        </pre>
      </details>
    </div>
  );
};

export default UserProfile;