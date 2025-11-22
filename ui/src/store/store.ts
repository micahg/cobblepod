import { configureStore } from '@reduxjs/toolkit';
import { backupApi } from '../services/backupApi';

export const store = configureStore({
  reducer: {
    [backupApi.reducerPath]: backupApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(backupApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
