import {
  Dialog,
  AppBar,
  Toolbar,
  IconButton,
  Typography,
  Slide,
  List,
  ListItem,
  ListItemText,
  Divider,
  Box,
  CircularProgress,
  Alert,
  Chip
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import RefreshIcon from '@mui/icons-material/Refresh';
import CancelIcon from '@mui/icons-material/Cancel';
import { type TransitionProps } from '@mui/material/transitions';
import React, { forwardRef } from 'react';
import { useGetJobItemsQuery, useCancelJobMutation } from '../../services/api';

const Transition = forwardRef(function Transition(
  props: TransitionProps & {
    children: React.ReactElement<unknown>;
  },
  ref: React.Ref<unknown>,
) {
  return <Slide direction="up" ref={ref} {...props} />;
});

interface JobItemsComponentProps {
  jobId: string | null;
  jobStatus?: string;
  failReason?: string;
  open: boolean;
  onClose: () => void;
}

const JobItemsComponent = ({ jobId, jobStatus, failReason, open, onClose }: JobItemsComponentProps) => {
  const { data, error, isLoading, refetch } = useGetJobItemsQuery(jobId || '', {
    skip: !jobId,
  });
  const [cancelJob, { isLoading: isCancelling }] = useCancelJobMutation();

  const handleCancel = async () => {
    if (jobId && window.confirm('Are you sure you want to cancel this job?')) {
      try {
        await cancelJob(jobId).unwrap();
        onClose();
      } catch (err) {
        console.error('Failed to cancel job', err);
      }
    }
  };

  const items = data?.items || [];

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'pending':
        return 'default';
      case 'downloading':
        return 'info';
      case 'processing':
        return 'warning';
      case 'uploading':
        return 'primary';
      case 'uploaded':
        return 'success';
      case 'synced':
        return 'success';
      case 'syncfailed':
        return 'error';
      case 'completed': // legacy: replaced by 'uploaded' in the backend
        return 'success';
      case 'skipped':
        return 'secondary';
      case 'failed':
        return 'error';
      default:
        return 'default';
    }
  };

  return (
    <Dialog
      fullScreen
      open={open}
      onClose={onClose}
      TransitionComponent={Transition}
    >
      <AppBar sx={{ position: 'relative' }}>
        <Toolbar>
          <IconButton
            edge="start"
            color="inherit"
            onClick={onClose}
            aria-label="close"
          >
            <CloseIcon />
          </IconButton>
          <Typography sx={{ ml: 2, flex: 1 }} variant="h6" component="div">
            Job Details {jobId && `- ${jobId.substring(0, 8)}...`}
          </Typography>
          <IconButton
            edge="end"
            color="inherit"
            onClick={handleCancel}
            disabled={isLoading || isCancelling || (jobStatus === 'completed' || jobStatus === 'failed')}
            aria-label="cancel job"
            sx={{ mr: 1 }}
          >
            <CancelIcon />
          </IconButton>
          <IconButton
            edge="end"
            color="inherit"
            onClick={() => refetch()}
            disabled={isLoading || (jobStatus === 'completed' || jobStatus === 'failed')}
            aria-label="refresh"
          >
            <RefreshIcon />
          </IconButton>
        </Toolbar>
      </AppBar>
      <Box sx={{ p: 2 }}>
        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
            <CircularProgress />
          </Box>
        ) : error ? (
          <Alert severity="error">Error loading job items</Alert>
        ) : (
          <Box>
            {failReason && (
              <Alert severity="error" sx={{ mb: 2 }}>
                Job Failed: {failReason}
              </Alert>
            )}
            {items.length === 0 ? (
              <Typography variant="body1" sx={{ p: 2, textAlign: 'center', color: 'text.secondary' }}>
                No items found for this job.
              </Typography>
            ) : (
              <List>
                {items.map((item, index) => (
                  <React.Fragment key={item.id}>
                    <ListItem alignItems="flex-start">
                  <ListItemText
                    primary={
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 0.5 }}>
                        <Typography variant="subtitle1" component="span" sx={{ fontWeight: 'bold' }}>
                          {item.title}
                        </Typography>
                        <Chip 
                          label={item.status} 
                          color={getStatusColor(item.status) as any} 
                          size="small" 
                        />
                      </Box>
                    }
                    secondary={
                      <React.Fragment>
                        <Typography
                          component="span"
                          variant="body2"
                          color="text.primary"
                          display="block"
                        >
                          Source: {item.source_url}
                        </Typography>
                        {item.error && (
                          <Typography
                            component="span"
                            variant="body2"
                            color="error"
                            display="block"
                            sx={{ mt: 0.5 }}
                          >
                            Error: {item.error}
                          </Typography>
                        )}
                        <Typography
                          component="span"
                          variant="caption"
                          color="text.secondary"
                          display="block"
                          sx={{ mt: 0.5 }}
                        >
                          Duration: {Math.round(item.duration / 1000000000)}s
                          {item.offset ? ` | Offset: ${Math.round(item.offset / 1000000000)}s` : ''}
                        </Typography>
                      </React.Fragment>
                    }
                  />
                </ListItem>
                {index < items.length - 1 && <Divider component="li" />}
              </React.Fragment>
            ))}
              </List>
            )}
          </Box>
        )}
      </Box>
    </Dialog>
  );
};

export default JobItemsComponent;
