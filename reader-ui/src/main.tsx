import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import 'animal-island-ui/style';
import './styles/global.css';
import { App } from './App';
import { ReaderAuthProvider } from './auth/ReaderAuthContext';

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <BrowserRouter>
      <ReaderAuthProvider>
        <App />
      </ReaderAuthProvider>
    </BrowserRouter>
  </React.StrictMode>
);
