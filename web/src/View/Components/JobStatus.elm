module View.Components.JobStatus exposing (canRun, view)

import Html exposing (Html, div, p, text)
import Html.Attributes exposing (class)
import Model exposing (MachineState, Msg)


{-| Jobs can be started when connected and the machine is idle.
-}
canRun : Maybe MachineState -> Bool
canRun machine =
    case machine of
        Just m ->
            m.connected
                && (case m.status of
                        Just st ->
                            st.state == "Idle" && not st.playing

                        Nothing ->
                            False
                   )

        Nothing ->
            False


view : Maybe MachineState -> Html Msg
view machine =
    case machine of
        Nothing ->
            p [ class "muted" ] [ text "Server unreachable" ]

        Just m ->
            if not m.connected then
                p [ class "muted" ] [ text "Not connected to a machine (see the Machine tab)" ]

            else
                jobLine m


jobLine : MachineState -> Html Msg
jobLine m =
    case m.job of
        Nothing ->
            text ""

        Just j ->
            let
                progress =
                    case ( j.phase, m.status ) of
                        ( "uploading", _ ) ->
                            " " ++ String.fromInt (percent j.sent j.total) ++ "%"

                        ( "playing", Just st ) ->
                            case st.progress of
                                line :: pct :: secs :: _ ->
                                    if st.playing then
                                        " line " ++ String.fromInt line ++ ", " ++ String.fromInt pct ++ "%, " ++ String.fromInt secs ++ "s"

                                    else
                                        " (waiting for machine: " ++ st.state ++ ")"

                                _ ->
                                    ""

                        _ ->
                            ""
            in
            div [ class ("job job-" ++ j.phase) ]
                [ text ("Job " ++ j.name ++ ": " ++ j.phase ++ progress)
                , case j.error of
                    Just e ->
                        div [ class "error" ] [ text e ]

                    Nothing ->
                        text ""
                ]


percent : Int -> Int -> Int
percent a b =
    if b == 0 then
        0

    else
        a * 100 // b
