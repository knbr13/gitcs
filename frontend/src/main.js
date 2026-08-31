import { mount } from 'svelte';
import './lib/tokens.css';
import App from './App.svelte';

mount(App, {
  target: document.getElementById('app')
});
