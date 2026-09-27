module View.Format exposing (fmt, toolName)


{-| Format a coordinate with up to three decimals.
-}
fmt : Float -> String
fmt v =
    String.fromFloat (toFloat (round (v * 1000)) / 1000)


toolName : Int -> String
toolName t =
    case t of
        0 ->
            "probe"

        8888 ->
            "laser"

        _ ->
            if t < 0 then
                "none"

            else
                String.fromInt t
