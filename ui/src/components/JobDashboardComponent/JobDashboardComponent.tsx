import { type SyntheticEvent, Fragment, useState } from 'react';
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
  Box,
  CircularProgress,
  Alert,
  Divider,
  IconButton
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import JobItemsComponent from '../JobItemsComponent/JobItemsComponent';

const JobDashboardComponent = () => {
  const [activeTab, setActiveTab] = useState<'active' | 'completed' | 'failed'>('active');
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  
  const queryStatus = activeTab === 'active' ? undefined : activeTab;

  const { data, error, isLoading } = useGetJobsQuery(queryStatus, {
    pollingInterval: 5000, // Poll every 5 seconds
  });

  const jobs = (data?.jobs || []).slice().reverse();

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const handleTabChange = (_event: SyntheticEvent, newValue: 'active' | 'completed' | 'failed') => {
    setActiveTab(newValue);
  };

  const handleOpenJobDetails = (jobId: string) => {
    console.log('Opening job details for job ID:', jobId);
    setSelectedJobId(jobId);
  };

  const handleCloseJobDetails = () => {
    setSelectedJobId(null);
  };

  return (
    <Card sx={{ 
      minWidth: 300, 
      maxWidth: 600, 
      width: '100%', 
      maxHeight: { xs: '50vh', md: 'min(600px, 80vh)' }, // we are stacked vertically with other components on xs
      display: 'flex', 
      flexDirection: 'column' 
    }}>
      <CardContent sx={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}>
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

        <Box sx={{ overflow: 'auto', flex: 1 }}>
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
                <Fragment key={job.id}>
                  <ListItem
                    alignItems="flex-start"
                    secondaryAction={
                      <IconButton 
                        edge="end" 
                        aria-label="info"
                        onClick={() => handleOpenJobDetails(job.id)}
                      >
                        <InfoIcon />
                      </IconButton>
                    }
                  >
                    <ListItemText
                      primary={
                        <Typography variant="subtitle1" component="span">
                          ID: {job.id.substring(0, 8)}...
                        </Typography>
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
                </Fragment>
              ))}
            </List>
          )}
        </Box>
      </CardContent>
      <JobItemsComponent 
        jobId={selectedJobId} 
        open={!!selectedJobId} 
        onClose={handleCloseJobDetails} 
      />
    </Card>
  );
};

export default JobDashboardComponent;
