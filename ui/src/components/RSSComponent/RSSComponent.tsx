import { useGetRSSQuery } from '../../services/api';
import { 
  Card, 
  CardContent, 
  Typography, 
  Box, 
  CircularProgress,
  Alert,
  Link
} from '@mui/material';
import RssFeedIcon from '@mui/icons-material/RssFeed';

const RSSComponent = () => {
  const { data, error, isLoading } = useGetRSSQuery();

  return (
    <Card sx={{ minWidth: 275, maxWidth: 500, width: '100%' }}>
      <CardContent>
        <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
          <RssFeedIcon color="primary" sx={{ mr: 1 }} />
          <Typography variant="h5" component="div">
            Your RSS Feed
          </Typography>
        </Box>

        {isLoading && (
          <Box sx={{ display: 'flex', justifyContent: 'center', my: 2 }}>
            <CircularProgress size={24} />
          </Box>
        )}

        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            Failed to load RSS feed.
          </Alert>
        )}

        {data?.error && (
           <Alert severity="warning" sx={{ mt: 2 }}>
            {data.error}
          </Alert>
        )}

        {data?.url && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              Subscribe to this URL in your podcast app:
            </Typography>
            <Box 
              sx={{ 
                p: 2, 
                bgcolor: 'grey.100', 
                borderRadius: 1,
                wordBreak: 'break-all',
                fontFamily: 'monospace',
                mt: 1
              }}
            >
              <Link href={data.url} target="_blank" rel="noopener noreferrer">
                {data.url}
              </Link>
            </Box>
          </Box>
        )}
      </CardContent>
    </Card>
  );
};

export default RSSComponent;
