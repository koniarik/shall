// Applies the reader's colour scheme before the page is drawn, so that the
// page never flashes the other one first. Without a choice, the stylesheet
// follows the system.
'use strict';

try {
  const scheme = localStorage.getItem('shall-scheme');
  if (scheme === 'dark' || scheme === 'light') document.documentElement.dataset.theme = scheme;
} catch {
  // Without storage, the system's scheme holds.
}
