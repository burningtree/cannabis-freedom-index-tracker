// The Astro dev proxy handles HTTP and WebSockets without depending on its port.
// Production stays opt-in; explicit API addresses override the local proxy.
export const submissionsAPI = import.meta.env.PUBLIC_SUBMISSIONS_API_URL
  || (import.meta.env.DEV ? "/__submissions" : "");
