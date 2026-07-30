import { useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import {
  IconButton,
  Menu,
  MenuItem,
  Avatar,
  Typography,
  Divider,
  ListItemIcon,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  Box,
} from '@mui/material';
import LogoutIcon from '@mui/icons-material/Logout';
import LinkIcon from '@mui/icons-material/Link';
import LinkOffIcon from '@mui/icons-material/LinkOff';
import { usePlayrunLoginMutation, usePlayrunLogoutMutation, useGetPlayrunStatusQuery } from '../services/api';

const AccountMenu = () => {
  const { user, isAuthenticated, logout } = useAuth0();
  const [playrunLogin] = usePlayrunLoginMutation();
  const [playrunLogout] = usePlayrunLogoutMutation();
  const { data: playrunStatus } = useGetPlayrunStatusQuery();

  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [playrunOpen, setPlayrunOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [playrunError, setPlayrunError] = useState<string | null>(null);

  if (!isAuthenticated || !user) {
    return null;
  }

  const handleOpen = (e: React.MouseEvent<HTMLElement>) => setAnchorEl(e.currentTarget);
  const handleClose = () => setAnchorEl(null);

  const handleLogout = () => {
    handleClose();
    playrunLogout();
    logout({ logoutParams: { returnTo: window.location.origin } });
  };

  const openPlayrun = () => {
    handleClose();
    setPlayrunOpen(true);
  };

  const handlePlayrunLogout = () => {
    handleClose();
    playrunLogout();
  };

  const closePlayrun = () => {
    setPlayrunOpen(false);
    setEmail('');
    setPassword('');
    setPlayrunError(null);
  };

  const handlePlayrunSubmit = async () => {
    setSubmitting(true);
    setPlayrunError(null);
    try {
      const result = await playrunLogin({ email, password }).unwrap();
      if (result.success) {
        closePlayrun();
      } else {
        setPlayrunError(result.error || 'Playrun login failed');
      }
    } catch (err: unknown) {
      const e = err as { data?: { error?: string } };
      setPlayrunError(e?.data?.error || 'Playrun login failed');
    } finally {
      setSubmitting(false);
    }
  };

  const initials = (user.name || user.email || '?')
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((s) => s[0]?.toUpperCase() || '')
    .join('') || '?';

  return (
    <Box>
      <IconButton onClick={handleOpen} aria-label="account menu" sx={{ p: 0 }}>
        <Avatar src={user.picture} alt={user.name || user.email || 'account'}>
          {initials}
        </Avatar>
      </IconButton>
      <Menu
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
        slotProps={{
          paper: { sx: { minWidth: 240, mt: 1.5 } },
        }}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <MenuItem disabled sx={{ flexDirection: 'column', alignItems: 'flex-start' }}>
          <Typography variant="subtitle1" component="div" sx={{ fontWeight: 500 }}>
            {user.name || user.email}
          </Typography>
          {user.name && user.email && (
            <Typography variant="body2" color="text.secondary">
              {user.email}
            </Typography>
          )}
        </MenuItem>
        <Divider />
        {playrunStatus?.loggedIn ? (
          <MenuItem onClick={handlePlayrunLogout}>
            <ListItemIcon>
              <LinkOffIcon fontSize="small" />
            </ListItemIcon>
            Disconnect Playrun{playrunStatus.email ? ` (${playrunStatus.email})` : ''}
          </MenuItem>
        ) : (
          <MenuItem onClick={openPlayrun}>
            <ListItemIcon>
              <LinkIcon fontSize="small" />
            </ListItemIcon>
            Connect Playrun
          </MenuItem>
        )}
        <Divider />
        <MenuItem onClick={handleLogout}>
          <ListItemIcon>
            <LogoutIcon fontSize="small" />
          </ListItemIcon>
          Log Out
        </MenuItem>
      </Menu>

      <Dialog open={playrunOpen} onClose={closePlayrun} maxWidth="xs" fullWidth>
        <DialogTitle>Connect Playrun Account</DialogTitle>
        <DialogContent>
          {playrunError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {playrunError}
            </Alert>
          )}
          <Box component="form" sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 1 }}>
            <TextField
              label="Playrun Email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
            />
            <TextField
              label="Playrun Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={closePlayrun} disabled={submitting}>
            Cancel
          </Button>
          <Button
            onClick={handlePlayrunSubmit}
            variant="contained"
            disabled={submitting || !email || !password}
          >
            {submitting ? 'Connecting...' : 'Connect'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default AccountMenu;