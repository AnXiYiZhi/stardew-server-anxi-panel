declare module 'qrcode' {
  export type QRCodeErrorCorrectionLevel = 'low' | 'medium' | 'quartile' | 'high' | 'L' | 'M' | 'Q' | 'H'

  export type QRCodeToDataURLOptions = {
    errorCorrectionLevel?: QRCodeErrorCorrectionLevel
    margin?: number
    scale?: number
    width?: number
    color?: {
      dark?: string
      light?: string
    }
  }

  export function toDataURL(text: string, options?: QRCodeToDataURLOptions): Promise<string>
}
