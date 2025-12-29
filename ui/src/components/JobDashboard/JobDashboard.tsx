import React, { useState } from 'react';
import { useGetJobsQuery } from '../../services/backupApi';
import styles from './JobDashboard.module.css';

const JobDashboard: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'active' | 'completed' | 'failed'>('active');
  
  const queryStatus = activeTab === 'active' ? undefined : activeTab;

  const { data, error, isLoading } = useGetJobsQuery(queryStatus, {
    pollingInterval: 5000, // Poll every 5 seconds
  });

  const jobs = data?.jobs || [];

  const getStatusClass = (status: string) => {
    switch (status.toLowerCase()) {
      case 'waiting':
        return styles['status-waiting'];
      case 'running':
        return styles['status-running'];
      case 'completed':
        return styles['status-completed'];
      case 'failed':
        return styles['status-failed'];
      default:
        return '';
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  return (
    <div className={styles.dashboardContainer}>
      <h2>Job Status</h2>
      <div className={styles.tabs}>
        <button 
          className={`${styles.tabButton} ${activeTab === 'active' ? styles.activeTab : ''}`}
          onClick={() => setActiveTab('active')}
        >
          Active
        </button>
        <button 
          className={`${styles.tabButton} ${activeTab === 'completed' ? styles.activeTab : ''}`}
          onClick={() => setActiveTab('completed')}
        >
          Completed
        </button>
        <button 
          className={`${styles.tabButton} ${activeTab === 'failed' ? styles.activeTab : ''}`}
          onClick={() => setActiveTab('failed')}
        >
          Failed
        </button>
      </div>

      {isLoading ? (
        <div className={styles.dashboardContainer}>Loading jobs...</div>
      ) : error ? (
        <div className={styles.dashboardContainer}>Error loading jobs</div>
      ) : jobs.length === 0 ? (
        <p>No {activeTab} jobs found.</p>
      ) : (
        <ul className={styles.jobList}>
          {jobs.map((job) => (
            <li key={job.id} className={styles.jobItem}>
              <div className={styles.jobHeader}>
                <span>ID: {job.id.substring(0, 8)}...</span>
                <span className={`${styles.jobStatus} ${getStatusClass(job.status)}`}>
                  {job.status}
                </span>
              </div>
              <div className={styles.jobTime}>
                Created: {formatDate(job.created_at)}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

export default JobDashboard;
