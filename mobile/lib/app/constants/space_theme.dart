import 'package:flutter/widgets.dart';

/// Spacing in logical pixels: S=4, M=8, L=12, Xl=16, Xxl=24, Xxxl=32.
abstract class AppSpace {
  static const EdgeInsets p0 = EdgeInsets.zero;
  static const EdgeInsets pS = EdgeInsets.all(4.0);
  static const EdgeInsets pM = EdgeInsets.all(8.0);
  static const EdgeInsets pL = EdgeInsets.all(12.0);
  static const EdgeInsets pXl = EdgeInsets.all(16.0);
  static const EdgeInsets pXxl = EdgeInsets.all(24.0);
  static const EdgeInsets pXxxl = EdgeInsets.all(32.0);

  static const EdgeInsets px0 = EdgeInsets.zero;
  static const EdgeInsets pxS = EdgeInsets.symmetric(horizontal: 4.0);
  static const EdgeInsets pxM = EdgeInsets.symmetric(horizontal: 8.0);
  static const EdgeInsets pxL = EdgeInsets.symmetric(horizontal: 12.0);
  static const EdgeInsets pxXl = EdgeInsets.symmetric(horizontal: 16.0);
  static const EdgeInsets pxXxl = EdgeInsets.symmetric(horizontal: 24.0);
  static const EdgeInsets pxXxxl = EdgeInsets.symmetric(horizontal: 32.0);

  static const EdgeInsets py0 = EdgeInsets.zero;
  static const EdgeInsets pyS = EdgeInsets.symmetric(vertical: 4.0);
  static const EdgeInsets pyM = EdgeInsets.symmetric(vertical: 8.0);
  static const EdgeInsets pyL = EdgeInsets.symmetric(vertical: 12.0);
  static const EdgeInsets pyXl = EdgeInsets.symmetric(vertical: 16.0);
  static const EdgeInsets pyXxl = EdgeInsets.symmetric(vertical: 24.0);
  static const EdgeInsets pyXxxl = EdgeInsets.symmetric(vertical: 32.0);

  static const EdgeInsets pr0 = EdgeInsets.zero;
  static const EdgeInsets prS = EdgeInsets.only(right: 4.0);
  static const EdgeInsets prM = EdgeInsets.only(right: 8.0);
  static const EdgeInsets prL = EdgeInsets.only(right: 12.0);
  static const EdgeInsets prXl = EdgeInsets.only(right: 16.0);
  static const EdgeInsets prXxl = EdgeInsets.only(right: 24.0);
  static const EdgeInsets prXxxl = EdgeInsets.only(right: 32.0);

  static const EdgeInsets pl0 = EdgeInsets.zero;
  static const EdgeInsets plS = EdgeInsets.only(left: 4.0);
  static const EdgeInsets plM = EdgeInsets.only(left: 8.0);
  static const EdgeInsets plL = EdgeInsets.only(left: 12.0);
  static const EdgeInsets plXl = EdgeInsets.only(left: 16.0);
  static const EdgeInsets plXxl = EdgeInsets.only(left: 24.0);
  static const EdgeInsets plXxxl = EdgeInsets.only(left: 32.0);

  static const EdgeInsets pt0 = EdgeInsets.zero;
  static const EdgeInsets ptS = EdgeInsets.only(top: 4.0);
  static const EdgeInsets ptM = EdgeInsets.only(top: 8.0);
  static const EdgeInsets ptL = EdgeInsets.only(top: 12.0);
  static const EdgeInsets ptXl = EdgeInsets.only(top: 16.0);
  static const EdgeInsets ptXxl = EdgeInsets.only(top: 24.0);
  static const EdgeInsets ptXxxl = EdgeInsets.only(top: 32.0);

  static const EdgeInsets pb0 = EdgeInsets.zero;
  static const EdgeInsets pbS = EdgeInsets.only(bottom: 4.0);
  static const EdgeInsets pbM = EdgeInsets.only(bottom: 8.0);
  static const EdgeInsets pbL = EdgeInsets.only(bottom: 12.0);
  static const EdgeInsets pbXl = EdgeInsets.only(bottom: 16.0);
  static const EdgeInsets pbXxl = EdgeInsets.only(bottom: 24.0);
  static const EdgeInsets pbXxxl = EdgeInsets.only(bottom: 32.0);

  static const SizedBox sx0 = SizedBox(width: 0.0);
  static const SizedBox sxS = SizedBox(width: 4.0);
  static const SizedBox sxM = SizedBox(width: 8.0);
  static const SizedBox sxL = SizedBox(width: 12.0);
  static const SizedBox sxXl = SizedBox(width: 16.0);
  static const SizedBox sxXxl = SizedBox(width: 24.0);
  static const SizedBox sxXxxl = SizedBox(width: 32.0);

  static const SizedBox sy0 = SizedBox(height: 0.0);
  static const SizedBox syS = SizedBox(height: 4.0);
  static const SizedBox syM = SizedBox(height: 8.0);
  static const SizedBox syL = SizedBox(height: 12.0);
  static const SizedBox syXl = SizedBox(height: 16.0);
  static const SizedBox syXxl = SizedBox(height: 24.0);
  static const SizedBox syXxxl = SizedBox(height: 32.0);
}
