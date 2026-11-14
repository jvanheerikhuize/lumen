import React from 'react';
import { createRoot } from 'react-dom/client';
import './app/globals.css';
import Home from './app/page';

const container = document.getElementById('root');
if (container) {
  createRoot(container).render(
    <React.StrictMode>
      <Home />
    </React.StrictMode>
  );
}
