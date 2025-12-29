import React, { useState } from 'react';
import { useGetJobsQuery } from '../../services/backupApi';
import {
  Card,
  CardContent,
  Typography,
  Tabs,
  Tab,
  List,
  ListItem,
  ListItemText,
  Chip,
  Box,
  CircularProgress,
  Alert,
  Divider
} from '@mui/material';

const JobDashboard: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'active' | 'completed' | 'failed'>('active');
  
  const queryStatus = activeTab === 'active' ? undefined : activeTab;

  const { data, error, isLoading } = useGetJobsQuery(queryStatus, {
    pollingInterval: 5000, // Poll every 5 seconds
  });

  const jobs = data?.jobs || [];

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'waiting':
        return 'warning';
      case 'running':
        return 'info';
      case 'completed':
        return 'success';
      case 'failed':
        return 'error';
      default:
        return 'default';
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const handleTabChange = (event: React.SyntheticEvent, newValue: 'active' | 'completed' | 'failed') => {
    setActiveTab(newValue);
  };

  return (
    <Card sx={{ minWidth: 300, maxWidth: 600, width: '100%', maxHeight: 600, overflow: 'auto' }}>
      <CardContent>
        <Typography variant="h5" component="div" gutterBottom>
          Job Status
        </Typography>
        
        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
          <Tabs value={activeTab} onChange={handleTabChange} aria-label="job status tabs">
            <Tab label="Active" value="active" />
            <Tab label="Completed" value="completed" />
            <Tab label="Failed" value="failed" />
          </Tabs>
        </Box>

        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
            <CircularProgress />
          </Box>
        ) : error ? (
          <Alert severity="error">Error loading jobs</Alert>
        ) : jobs.length === 0 ? (
          <Typography variant="body1" sx={{ p: 2, textAlign: 'center', color: 'text.secondary' }}>
            No {activeTab} jobs found.
          </Typography>
        ) : (
          <List sx={{ width: '100%', bgcolor: 'background.paper' }}>
            {jobs.map((job, index) => (
              <React.Fragment key={job.id}>
                <ListItem alignItems="flex-start">
                  <ListItemText
                    primary={
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <Typography variant="subtitle1" component="span">
                          ID: {job.id.substring(0, 8)}...
                        </Typography>
                        <Chip 
                          label={job.status} 
                          color={getStatusColor(job.status) as any} 
                          size="small" 
                        />
                      </Box>
                    }
                    secondary={
                      <Typography
                        sx={{ display: 'inline' }}
                        component="span"
                        variant="body2"
                        color="text.primary"
                      >
                        Created: {formatDate(job.created_at)}
                      </Typography>
                    }
                  />
                </ListItem>
                {index < jobs.length - 1 && <Divider variant="inset" component="li" />}
              </React.Fragment>
            ))}
          </List>
        )}
      </CardContent>
    </Card>
  );
};

export default JobDashboard;
