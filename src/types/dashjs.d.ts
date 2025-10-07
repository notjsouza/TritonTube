declare module 'dashjs' {
  interface MediaPlayer {
    create(): MediaPlayer;
    initialize(element: HTMLVideoElement, url: string, autoPlay: boolean): void;
    destroy(): void;
  }
  
  export function MediaPlayer(): MediaPlayer;
}
